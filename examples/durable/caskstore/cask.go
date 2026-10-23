package caskstore

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dmundt/go-cask/cas"
	"github.com/dmundt/go-cask/cas/backend/fs"
	"github.com/dmundt/go-cask/cas/codec/json"
	"github.com/dmundt/go-cask/cas/hash/sha256"
	"github.com/dmundt/go-cask/cas/refs"
	"github.com/dmundt/go-mvu/store"
)

type record[M any] struct {
	Version uint64 `json:"version"`
	Model   M      `json:"model"`
}

func (*record[M]) Type() string             { return "go-mvu.model@1" }
func (*record[M]) References() []cas.Digest { return nil }

type Store[M any] struct {
	mu      sync.Mutex
	objects *cas.Store[*record[M]]
	refs    *refs.Store
}

func Open[M any](root string) (*Store[M], error) {
	if root == "" {
		return nil, errors.New("CASK store directory is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create CASK store directory: %w", err)
	}
	objects, err := fs.New(filepath.Join(root, "objects"), fs.WithDirSync())
	if err != nil {
		return nil, fmt.Errorf("open CASK object backend: %w", err)
	}
	objectStore := cas.New[*record[M]](objects, json.New[*record[M]](), sha256.New())
	refStore, err := refs.Open(filepath.Join(root, "refs"))
	if err != nil {
		if closeErr := objectStore.Close(); closeErr != nil {
			return nil, fmt.Errorf("open CASK refs: %w (close backend: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("open CASK refs: %w", err)
	}
	return &Store[M]{objects: objectStore, refs: refStore}, nil
}

func (s *Store[M]) Load(ctx context.Context, key string) (store.Snapshot[M], error) {
	if err := ctx.Err(); err != nil {
		return store.Snapshot[M]{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	digest, err := s.refs.Get(ctx, refName(key))
	if errors.Is(err, refs.ErrNotFound) {
		return store.Snapshot[M]{}, store.ErrNotFound
	}
	if err != nil {
		return store.Snapshot[M]{}, fmt.Errorf("read CASK model ref: %w", err)
	}
	value, err := s.objects.Get(ctx, digest)
	if err != nil {
		return store.Snapshot[M]{}, fmt.Errorf("read CASK model object: %w", err)
	}
	if value.Version == 0 {
		return store.Snapshot[M]{}, errors.New("invalid CASK model version")
	}
	return store.Snapshot[M]{
		Model: value.Model, Version: store.Version(value.Version),
	}, nil
}

func (s *Store[M]) Create(
	ctx context.Context,
	key string,
	model M,
) (store.Snapshot[M], error) {
	return s.write(ctx, key, 0, model, true)
}

func (s *Store[M]) CompareAndSwap(
	ctx context.Context,
	key string,
	version store.Version,
	model M,
) (store.Snapshot[M], error) {
	return s.write(ctx, key, version, model, false)
}

func (s *Store[M]) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name := refName(key)
	if err := refs.ValidateName(name); err != nil {
		return fmt.Errorf("invalid CASK model key: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refs.Delete(ctx, name); err != nil {
		return fmt.Errorf("delete CASK model ref: %w", err)
	}
	return nil
}

func (s *Store[M]) Keys() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	listed, err := s.refs.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list CASK model refs: %w", err)
	}
	keys := make([]string, 0, len(listed))
	for _, ref := range listed {
		if !strings.HasPrefix(ref.Name, "models/") {
			continue
		}
		encoded := strings.TrimPrefix(ref.Name, "models/")
		key, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || base64.RawURLEncoding.EncodeToString(key) != encoded {
			return nil, fmt.Errorf("invalid CASK model ref name %q", ref.Name)
		}
		keys = append(keys, string(key))
	}
	return keys, nil
}

func (s *Store[M]) Close() error {
	if err := s.objects.Close(); err != nil {
		return fmt.Errorf("close CASK object store: %w", err)
	}
	return nil
}

func (s *Store[M]) write(
	ctx context.Context,
	key string,
	version store.Version,
	model M,
	create bool,
) (store.Snapshot[M], error) {
	if err := ctx.Err(); err != nil {
		return store.Snapshot[M]{}, err
	}
	if version == ^store.Version(0) {
		return store.Snapshot[M]{}, errors.New("CASK model version exhausted")
	}
	name := refName(key)
	if err := refs.ValidateName(name); err != nil {
		return store.Snapshot[M]{}, fmt.Errorf("invalid CASK model key: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	currentDigest, err := s.refs.Get(ctx, name)
	currentVersion := store.Version(0)
	switch {
	case errors.Is(err, refs.ErrNotFound):
		if create && version != 0 {
			return store.Snapshot[M]{}, store.ErrConflict
		}
		if !create && version != 0 {
			return store.Snapshot[M]{}, store.ErrConflict
		}
	case err != nil:
		return store.Snapshot[M]{}, fmt.Errorf("read CASK model ref: %w", err)
	default:
		if create {
			return store.Snapshot[M]{}, store.ErrAlreadyExists
		}
		current, err := s.objects.Get(ctx, currentDigest)
		if err != nil {
			return store.Snapshot[M]{}, fmt.Errorf("read current CASK model: %w", err)
		}
		if current.Version == 0 {
			return store.Snapshot[M]{}, errors.New("invalid current CASK model version")
		}
		currentVersion = store.Version(current.Version)
		if currentVersion != version {
			return store.Snapshot[M]{}, store.ErrConflict
		}
	}

	next := version + 1
	digest, err := s.objects.Put(ctx, &record[M]{Version: uint64(next), Model: model})
	if err != nil {
		return store.Snapshot[M]{}, fmt.Errorf("write CASK model object: %w", err)
	}
	if err := s.refs.Set(ctx, name, digest); err != nil {
		return store.Snapshot[M]{}, fmt.Errorf("publish CASK model ref (state may be committed): %w", err)
	}
	return store.Snapshot[M]{Model: model, Version: next}, nil
}

func refName(key string) string {
	return "models/" + base64.RawURLEncoding.EncodeToString([]byte(key))
}

var _ store.Store[string, struct{}] = (*Store[struct{}])(nil)
