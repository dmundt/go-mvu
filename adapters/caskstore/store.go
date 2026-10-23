// Package caskstore persists versioned models in a single-host CASK repository.
// One Store owns an exclusive OS lock for its directory until Close.
package caskstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/dmundt/go-cask/cas"
	caskfs "github.com/dmundt/go-cask/cas/backend/fs"
	"github.com/dmundt/go-cask/cas/refs"
	"github.com/dmundt/go-mvu/store"
	"github.com/gofrs/flock"
)

var (
	ErrLocked          = errors.New("caskstore: directory already in use")
	ErrClosed          = errors.New("caskstore: closed")
	ErrSchema          = errors.New("caskstore: unsupported schema")
	ErrCommitUncertain = errors.New("caskstore: publication failed; commit may have occurred")
)

type record[M any] struct {
	Schema  int           `json:"schema"`
	Version store.Version `json:"version"`
	Deleted bool          `json:"deleted,omitempty"`
	Model   M             `json:"model"`
}

// Store uses JSON records and SHA-256 identities. Models must support a lossless
// JSON round trip. Schema changes require an explicit application migration.
// Old objects and reflogs remain on disk; Delete is logical, not secure erasure.
type Store[M any] struct {
	mu      sync.Mutex
	backend *caskfs.Backend
	heads   *refs.Store
	lock    *flock.Flock
	closed  bool
}

// Open creates or reopens a repository. It rejects a second owner, including in
// another process. Use local persistent storage, not shared/network filesystems.
func Open[M any](dir string) (*Store[M], error) {
	if dir == "" {
		return nil, errors.New("caskstore: empty directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("caskstore: create directory: %w", err)
	}
	lock := flock.New(filepath.Join(dir, ".gomvu.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("caskstore: lock: %w", err)
	}
	if !ok {
		return nil, ErrLocked
	}
	backend, err := caskfs.New(filepath.Join(dir, "objects"), caskfs.WithDirSync())
	if err != nil {
		return nil, errors.Join(err, lock.Unlock())
	}
	heads, err := refs.Open(filepath.Join(dir, "heads"))
	if err != nil {
		return nil, errors.Join(err, lock.Unlock())
	}
	return &Store[M]{backend: backend, heads: heads, lock: lock}, nil
}

func keyName(key string) (string, error) {
	if key == "" {
		return "", errors.New("caskstore: empty key")
	}
	sum := sha256.Sum256([]byte(key))
	return "models/" + hex.EncodeToString(sum[:]), nil
}

func (s *Store[M]) read(ctx context.Context, name string) (record[M], error) {
	var value record[M]
	digest, err := s.heads.Get(ctx, name)
	if err != nil {
		if errors.Is(err, refs.ErrNotFound) {
			return value, store.ErrNotFound
		}
		return value, err
	}
	reader, err := s.backend.Get(ctx, digest)
	if err != nil {
		return value, fmt.Errorf("caskstore: read referenced object: %w", err)
	}
	data, readErr := io.ReadAll(reader)
	if err := errors.Join(readErr, reader.Close()); err != nil {
		return value, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != digest.String() {
		return value, errors.New("caskstore: object digest mismatch")
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("caskstore: decode: %w", err)
	}
	if value.Schema != 1 || value.Version == 0 {
		return value, ErrSchema
	}
	return value, nil
}

func (s *Store[M]) ready(ctx context.Context) error {
	if s.closed {
		return ErrClosed
	}
	return ctx.Err()
}

func (s *Store[M]) Load(ctx context.Context, key string) (store.Snapshot[M], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var empty store.Snapshot[M]
	if err := s.ready(ctx); err != nil {
		return empty, err
	}
	name, err := keyName(key)
	if err != nil {
		return empty, err
	}
	value, err := s.read(ctx, name)
	if err != nil {
		return empty, err
	}
	if value.Deleted {
		return empty, store.ErrNotFound
	}
	return store.Snapshot[M]{Model: value.Model, Version: value.Version}, nil
}

func (s *Store[M]) write(
	ctx context.Context, name string, value record[M],
) (store.Snapshot[M], error) {
	var snapshot store.Snapshot[M]
	data, err := json.Marshal(value)
	if err != nil {
		return snapshot, fmt.Errorf("caskstore: encode: %w", err)
	}
	// Decode before committing: returned models must not alias callers' maps
	// or pointers, and unsupported round trips must fail before persistence.
	var copy record[M]
	if err := json.Unmarshal(data, &copy); err != nil {
		return snapshot, fmt.Errorf("caskstore: round trip: %w", err)
	}
	sum := sha256.Sum256(data)
	digest, err := cas.ParseDigest(hex.EncodeToString(sum[:]))
	if err != nil {
		return snapshot, err
	}
	if err := s.backend.Put(ctx, digest, bytes.NewReader(data)); err != nil {
		return snapshot, fmt.Errorf("caskstore: write object: %w", err)
	}
	if err := s.heads.Set(ctx, name, digest); err != nil {
		// A ref publication/reflog failure may occur after publication. Never
		// report success or automatically replay this uncertain write.
		return snapshot, errors.Join(ErrCommitUncertain, err)
	}
	return store.Snapshot[M]{Model: copy.Model, Version: copy.Version}, nil
}

func (s *Store[M]) mutate(
	ctx context.Context, key string, version store.Version, model M, create bool,
) (store.Snapshot[M], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var empty store.Snapshot[M]
	if err := s.ready(ctx); err != nil {
		return empty, err
	}
	name, err := keyName(key)
	if err != nil {
		return empty, err
	}
	current, err := s.read(ctx, name)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return empty, err
	}
	exists := err == nil && !current.Deleted
	if create && exists {
		return empty, store.ErrAlreadyExists
	}
	if !create && ((!exists && version != 0) || (exists && version != current.Version)) {
		return empty, store.ErrConflict
	}
	if create {
		version = 0
	}
	if version == math.MaxUint64 {
		return empty, errors.New("caskstore: version exhausted")
	}
	return s.write(ctx, name, record[M]{Schema: 1, Version: version + 1, Model: model})
}

func (s *Store[M]) Create(ctx context.Context, key string, model M) (store.Snapshot[M], error) {
	return s.mutate(ctx, key, 0, model, true)
}

func (s *Store[M]) CompareAndSwap(
	ctx context.Context, key string, version store.Version, model M,
) (store.Snapshot[M], error) {
	return s.mutate(ctx, key, version, model, false)
}

func (s *Store[M]) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(ctx); err != nil {
		return err
	}
	name, err := keyName(key)
	if err != nil {
		return err
	}
	current, err := s.read(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if current.Deleted {
		return nil
	}
	_, err = s.write(ctx, name, record[M]{
		Schema: 1, Version: current.Version, Deleted: true,
	})
	return err
}

// Close releases directory ownership after all in-flight operations finish.
func (s *Store[M]) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if err := s.lock.Unlock(); err != nil {
		return fmt.Errorf("caskstore: unlock: %w", err)
	}
	s.closed = true
	return nil
}
