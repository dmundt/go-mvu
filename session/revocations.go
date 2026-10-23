package session

import (
	"errors"
	"sync"
	"time"
)

var ErrRevocationCapacity = errors.New("session revocation capacity exhausted")

type revocation struct {
	expires time.Time
	done    chan struct{}
	pending bool
}

// MemoryRevocations is a concurrency-safe, bounded, single-process revocation
// backend. It retains revoked IDs only until their signed expiry. Expired entries
// are cleaned on access; no goroutine or shutdown is needed. Pending migrations
// reserve capacity and are never evicted. When full, lifecycle operations fail
// closed before invoking migration or changing cookies.
//
// Share the same instance among all middleware verifying the same sessions.
// Replacing it (including process restart) loses revocations; it is not suitable
// for cross-process logout guarantees. Use short lifetimes and coordinated
// signing-key retirement when that limitation is unacceptable.
// The public API currently accepts this concrete backend only; a pluggable
// revocation interface and distributed backends are deferred.
type MemoryRevocations struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]*revocation
}

func NewMemoryRevocations(capacity int) (*MemoryRevocations, error) {
	if capacity <= 0 {
		return nil, ErrInvalidOptions
	}
	return &MemoryRevocations{capacity: capacity, entries: make(map[string]*revocation)}, nil
}

func (memory *MemoryRevocations) cleanup(now time.Time) {
	for id, entry := range memory.entries {
		if !entry.pending && !now.Before(entry.expires) {
			delete(memory.entries, id)
		}
	}
}

func (memory *MemoryRevocations) revoked(id string, clock func() time.Time) bool {
	for {
		memory.mu.Lock()
		memory.cleanup(clock())
		entry, exists := memory.entries[id]
		if !exists || !entry.pending {
			memory.mu.Unlock()
			return exists
		}
		memory.mu.Unlock()
		<-entry.done
	}
}

func (memory *MemoryRevocations) transition(current token, clock func() time.Time, migrate func() error) (err error) {
	for {
		memory.mu.Lock()
		now := clock()
		memory.cleanup(now)
		if !now.Before(current.expires) {
			memory.mu.Unlock()
			return ErrInvalidSession
		}
		if entry, exists := memory.entries[current.id]; exists {
			pending := entry.pending
			memory.mu.Unlock()
			if !pending {
				return ErrInvalidSession
			}
			<-entry.done
			continue
		}
		if len(memory.entries) >= memory.capacity {
			memory.mu.Unlock()
			return ErrRevocationCapacity
		}
		entry := &revocation{expires: current.expires, done: make(chan struct{}), pending: true}
		memory.entries[current.id] = entry
		memory.mu.Unlock()
		succeeded := false
		defer func() {
			memory.mu.Lock()
			if succeeded {
				entry.pending = false
			} else {
				delete(memory.entries, current.id)
			}
			close(entry.done)
			memory.mu.Unlock()
		}()
		if migrate != nil {
			if err := migrate(); err != nil {
				return err
			}
		}
		succeeded = true
		return nil
	}
}
