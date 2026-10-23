package jsonstore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dmundt/go-mvu/store"
)

var (
	ErrClosed = errors.New("jsonstore: closed")
	ErrSchema = errors.New("jsonstore: unsupported schema")
)

type record[M any] struct {
	Schema  int           `json:"schema"`
	Version store.Version `json:"version"`
	Model   M             `json:"model"`
}

// Store persists one JSON record per key. It serializes operations within one
// process; it does not coordinate writers in other processes.
type Store[M any] struct {
	mu     sync.Mutex
	dir    string
	closed bool
}

func Open[M any](dir string) (*Store[M], error) {
	if dir == "" {
		return nil, errors.New("jsonstore: empty directory")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("jsonstore: create directory: %w", err)
	}
	return &Store[M]{dir: dir}, nil
}

func (s *Store[M]) ready(ctx context.Context) error {
	if s.closed {
		return ErrClosed
	}
	return ctx.Err()
}

func fileName(key string) (string, error) {
	if key == "" {
		return "", errors.New("jsonstore: empty key")
	}
	return base64.RawURLEncoding.EncodeToString([]byte(key)) + ".json", nil
}

func (s *Store[M]) read(name string) (record[M], error) {
	var value record[M]
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return value, store.ErrNotFound
	}
	if err != nil {
		return value, fmt.Errorf("jsonstore: read record: %w", err)
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("jsonstore: decode record: %w", err)
	}
	if value.Schema != 1 || value.Version == 0 {
		return value, ErrSchema
	}
	return value, nil
}

func (s *Store[M]) Load(ctx context.Context, key string) (store.Snapshot[M], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(ctx); err != nil {
		return store.Snapshot[M]{}, err
	}
	name, err := fileName(key)
	if err != nil {
		return store.Snapshot[M]{}, err
	}
	value, err := s.read(name)
	if err != nil {
		return store.Snapshot[M]{}, err
	}
	return store.Snapshot[M]{Model: value.Model, Version: value.Version}, nil
}

func (s *Store[M]) Create(
	ctx context.Context,
	key string,
	model M,
) (store.Snapshot[M], error) {
	return s.mutate(ctx, key, 0, model, true)
}

func (s *Store[M]) CompareAndSwap(
	ctx context.Context,
	key string,
	version store.Version,
	model M,
) (store.Snapshot[M], error) {
	return s.mutate(ctx, key, version, model, false)
}

func (s *Store[M]) mutate(
	ctx context.Context,
	key string,
	version store.Version,
	model M,
	create bool,
) (store.Snapshot[M], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(ctx); err != nil {
		return store.Snapshot[M]{}, err
	}
	name, err := fileName(key)
	if err != nil {
		return store.Snapshot[M]{}, err
	}
	current, err := s.read(name)
	exists := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.Snapshot[M]{}, err
	}
	if create && exists {
		return store.Snapshot[M]{}, store.ErrAlreadyExists
	}
	if !create && ((!exists && version != 0) || (exists && version != current.Version)) {
		return store.Snapshot[M]{}, store.ErrConflict
	}
	if create {
		version = 0
	}
	if version == math.MaxUint64 {
		return store.Snapshot[M]{}, errors.New("jsonstore: version exhausted")
	}
	next := record[M]{Schema: 1, Version: version + 1, Model: model}
	data, err := json.Marshal(next)
	if err != nil {
		return store.Snapshot[M]{}, fmt.Errorf("jsonstore: encode record: %w", err)
	}
	var copy record[M]
	if err := json.Unmarshal(data, &copy); err != nil {
		return store.Snapshot[M]{}, fmt.Errorf("jsonstore: round trip record: %w", err)
	}
	if err := s.write(name, data); err != nil {
		return store.Snapshot[M]{}, err
	}
	return store.Snapshot[M]{Model: copy.Model, Version: copy.Version}, nil
}

func (s *Store[M]) write(name string, data []byte) error {
	file, err := os.CreateTemp(s.dir, ".pending-*")
	if err != nil {
		return fmt.Errorf("jsonstore: create temporary record: %w", err)
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("jsonstore: set record permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("jsonstore: write record: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("jsonstore: sync record: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("jsonstore: close record: %w", err)
	}
	if err := os.Rename(temp, filepath.Join(s.dir, name)); err != nil {
		return fmt.Errorf("jsonstore: publish record: %w", err)
	}
	return nil
}

func (s *Store[M]) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(ctx); err != nil {
		return err
	}
	name, err := fileName(key)
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(s.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("jsonstore: delete record: %w", err)
	}
	return nil
}

func (s *Store[M]) Keys() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("jsonstore: list records: %w", err)
	}
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		encoded := strings.TrimSuffix(name, ".json")
		key, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || base64.RawURLEncoding.EncodeToString(key) != encoded {
			return nil, fmt.Errorf("jsonstore: invalid record filename %q", name)
		}
		keys = append(keys, string(key))
	}
	return keys, nil
}

func (s *Store[M]) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

var _ store.Store[string, struct{}] = (*Store[struct{}])(nil)
