package caskstore

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dmundt/go-mvu/store"
)

func TestStorePersistsCASAndBinaryKeys(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "models")
	key := string([]byte{0xff, 0x80, 0x01})
	first, err := Open[int](path)
	if err != nil {
		t.Fatal(err)
	}
	created, err := first.Create(ctx, key, 4)
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 || created.Model != 4 {
		t.Fatalf("created snapshot = %+v", created)
	}
	if _, err := first.CompareAndSwap(ctx, key, 0, 5); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("stale CAS error = %v, want conflict", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := Open[int](path)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	snapshot, err := restarted.Load(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Version != 1 || snapshot.Model != 4 {
		t.Fatalf("reloaded snapshot = %+v", snapshot)
	}
	keys, err := restarted.Keys()
	if err != nil || len(keys) != 1 || keys[0] != key {
		t.Fatalf("keys = %q, error = %v", keys, err)
	}
}

func TestStoreConcurrentCAS(t *testing.T) {
	ctx := context.Background()
	models, err := Open[int](filepath.Join(t.TempDir(), "models"))
	if err != nil {
		t.Fatal(err)
	}
	defer models.Close()
	if _, err := models.Create(ctx, "counter", 0); err != nil {
		t.Fatal(err)
	}

	const workers = 4
	const increments = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range increments {
				for {
					snapshot, err := models.Load(ctx, "counter")
					if err != nil {
						errs <- err
						return
					}
					_, err = models.CompareAndSwap(ctx, "counter", snapshot.Version, snapshot.Model+1)
					if errors.Is(err, store.ErrConflict) {
						continue
					}
					if err != nil {
						errs <- err
						return
					}
					break
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	snapshot, err := models.Load(ctx, "counter")
	if err != nil {
		t.Fatal(err)
	}
	if want := workers * increments; snapshot.Model != want {
		t.Fatalf("final count = %d, want %d", snapshot.Model, want)
	}
}
