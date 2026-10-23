package core

import (
	"context"
	"errors"
	"sync"
)

type Version uint64

type Snapshot[M any] struct {
	Model   M
	Version Version
}

// Store persists models with optimistic concurrency.
type Store[K comparable, M any] interface {
	Load(context.Context, K) (Snapshot[M], error)
	// CompareAndSwap stores model when the current version equals version.
	// Version 0 on a missing key creates it.
	CompareAndSwap(context.Context, K, Version, M) (Snapshot[M], error)
	// Create stores model at version 1 or fails with ErrAlreadyExists.
	Create(context.Context, K, M) (Snapshot[M], error)
	// Delete removes key. Deleting a missing key is not an error.
	Delete(context.Context, K) error
}

var (
	ErrNotFound      = errors.New("model not found")
	ErrAlreadyExists = errors.New("model already exists")
	ErrConflict      = errors.New("model version conflict")
)

type memoryEntry[M any] struct {
	model   M
	version Version
}

// Memory is a race-safe in-memory Store. Locks cover only map access; callers
// render and execute commands without holding them.
//
// Models are stored by value. Maps, slices, or pointers share memory with
// callers; treat loaded models as immutable (copy before mutating in Update)
// or use NewMemoryWithClone, which copies on every write and read.
type Memory[K comparable, M any] struct {
	mu      sync.RWMutex
	entries map[K]memoryEntry[M]
	clone   func(M) M
}

func NewMemory[K comparable, M any]() *Memory[K, M] {
	return &Memory[K, M]{entries: make(map[K]memoryEntry[M])}
}

// NewMemoryWithClone copies models with clone on every write and read so
// callers never share maps, slices, or pointers with the store.
func NewMemoryWithClone[K comparable, M any](clone func(M) M) *Memory[K, M] {
	memory := NewMemory[K, M]()
	memory.clone = clone
	return memory
}

func (m *Memory[K, M]) copy(model M) M {
	if m.clone == nil {
		return model
	}
	return m.clone(model)
}

func (m *Memory[K, M]) Load(ctx context.Context, key K) (Snapshot[M], error) {
	if err := ctx.Err(); err != nil {
		return Snapshot[M]{}, err
	}
	m.mu.RLock()
	entry, ok := m.entries[key]
	m.mu.RUnlock()
	if !ok {
		return Snapshot[M]{}, ErrNotFound
	}
	return Snapshot[M]{Model: m.copy(entry.model), Version: entry.version}, nil
}

func (m *Memory[K, M]) Create(ctx context.Context, key K, model M) (Snapshot[M], error) {
	if err := ctx.Err(); err != nil {
		return Snapshot[M]{}, err
	}
	stored := m.copy(model)
	m.mu.Lock()
	if _, exists := m.entries[key]; exists {
		m.mu.Unlock()
		return Snapshot[M]{}, ErrAlreadyExists
	}
	m.entries[key] = memoryEntry[M]{model: stored, version: 1}
	m.mu.Unlock()
	return Snapshot[M]{Model: m.copy(stored), Version: 1}, nil
}

func (m *Memory[K, M]) CompareAndSwap(
	ctx context.Context,
	key K,
	version Version,
	model M,
) (Snapshot[M], error) {
	if err := ctx.Err(); err != nil {
		return Snapshot[M]{}, err
	}
	stored := m.copy(model)
	m.mu.Lock()
	entry, exists := m.entries[key]
	if (!exists && version != 0) || (exists && entry.version != version) {
		m.mu.Unlock()
		return Snapshot[M]{}, ErrConflict
	}
	next := memoryEntry[M]{model: stored, version: version + 1}
	m.entries[key] = next
	m.mu.Unlock()
	return Snapshot[M]{Model: m.copy(stored), Version: next.version}, nil
}

func (m *Memory[K, M]) Delete(ctx context.Context, key K) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.entries, key)
	m.mu.Unlock()
	return nil
}
