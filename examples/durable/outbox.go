package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dmundt/go-mvu/core"
	"github.com/dmundt/go-mvu/internal/example/jsonstore"
)

type FileSink struct {
	dir string
	mu  sync.Mutex
}

func NewFileSink(dir string) (*FileSink, error) {
	if dir == "" {
		return nil, errors.New("delivery directory is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create delivery directory: %w", err)
	}
	return &FileSink{dir: dir}, nil
}

func (sink *FileSink) Deliver(ctx context.Context, event OutboxEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.ID == "" {
		return errors.New("outbox event ID is required")
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	path := hashedPath(sink.dir, event.ID)
	if data, err := os.ReadFile(path); err == nil {
		var delivered OutboxEvent
		if err := json.Unmarshal(data, &delivered); err != nil {
			return fmt.Errorf("decode existing delivery: %w", err)
		}
		if delivered.ID != event.ID {
			return errors.New("delivery idempotency key collision")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing delivery: %w", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode delivery: %w", err)
	}
	if err := writeAtomic(path, data); err != nil {
		return fmt.Errorf("persist delivery: %w", err)
	}
	return nil
}

type outboxWorker struct {
	store *jsonstore.Store[CounterModel]
	sink  *FileSink
}

func (worker *outboxWorker) Drain(ctx context.Context, key string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		snapshot, err := worker.store.Load(ctx, key)
		if err != nil {
			if errors.Is(err, core.ErrNotFound) {
				return nil
			}
			return err
		}
		if len(snapshot.Model.Outbox) == 0 {
			return nil
		}
		event := snapshot.Model.Outbox[0]
		if err := worker.sink.Deliver(ctx, event); err != nil {
			return err
		}
		next := snapshot.Model
		next.Outbox = append([]OutboxEvent(nil), next.Outbox[1:]...)
		if _, err := worker.store.CompareAndSwap(ctx, key, snapshot.Version, next); err != nil {
			if errors.Is(err, core.ErrConflict) {
				continue
			}
			return err
		}
	}
}

func (worker *outboxWorker) Run(ctx context.Context, wakes <-chan string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case key, ok := <-wakes:
			if !ok {
				return
			}
			worker.drainAndLog(ctx, key)
		case <-ticker.C:
			keys, err := worker.store.Keys()
			if err != nil {
				fmt.Fprintf(os.Stderr, "list outbox records: %v\n", err)
				continue
			}
			for _, key := range keys {
				worker.drainAndLog(ctx, key)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (worker *outboxWorker) drainAndLog(ctx context.Context, key string) {
	if err := worker.Drain(ctx, key); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "deliver outbox for %q: %v\n", key, err)
	}
}

func hashedPath(dir, key string) string {
	return filepath.Join(dir, hashKey(key)+".json")
}

func hashKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func writeAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".pending-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
