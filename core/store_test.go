package core

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestMemoryCompareAndSwap(t *testing.T) {
	ctx := context.Background()
	memory := NewMemory[string, int]()
	if _, err := memory.Load(ctx, "key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
	first, err := memory.CompareAndSwap(ctx, "key", 0, 10)
	if err != nil {
		t.Fatalf("initial CompareAndSwap() error = %v", err)
	}
	if first.Version != 1 || first.Model != 10 {
		t.Fatalf("initial snapshot = %+v", first)
	}
	if _, err := memory.CompareAndSwap(ctx, "key", 0, 20); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale CompareAndSwap() error = %v, want ErrConflict", err)
	}
	next, err := memory.CompareAndSwap(ctx, "key", first.Version, 11)
	if err != nil || next.Version != 2 || next.Model != 11 {
		t.Fatalf("next snapshot = %+v, error = %v", next, err)
	}
}

func TestMemoryCreateLoadDelete(t *testing.T) {
	ctx := context.Background()
	memory := NewMemory[string, int]()
	created, err := memory.Create(ctx, "k", 5)
	if err != nil || created.Version != 1 || created.Model != 5 {
		t.Fatalf("Create = %+v, %v", created, err)
	}
	if _, err := memory.Create(ctx, "k", 6); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate Create error = %v", err)
	}
	if _, err := memory.CompareAndSwap(ctx, "missing", 3, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("CAS on missing key error = %v", err)
	}
	if err := memory.Delete(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if _, err := memory.Load(ctx, "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load after Delete error = %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := memory.Load(cancelled, "k"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Load error = %v", err)
	}
}

func TestMemoryConcurrentCompareAndSwapLosesNoUpdates(t *testing.T) {
	ctx := context.Background()
	memory := NewMemory[string, int]()
	if _, err := memory.Create(ctx, "n", 0); err != nil {
		t.Fatal(err)
	}
	const workers, increments = 8, 50
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range increments {
				for {
					snapshot, err := memory.Load(ctx, "n")
					if err != nil {
						t.Error(err)
						return
					}
					_, err = memory.CompareAndSwap(ctx, "n", snapshot.Version, snapshot.Model+1)
					if err == nil {
						break
					}
					if !errors.Is(err, ErrConflict) {
						t.Error(err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	final, _ := memory.Load(ctx, "n")
	if final.Model != workers*increments {
		t.Fatalf("final = %d, want %d", final.Model, workers*increments)
	}
}

func TestMemoryWithCloneIsolatesCallers(t *testing.T) {
	ctx := context.Background()
	memory := NewMemoryWithClone[string, []int](func(values []int) []int {
		return append([]int(nil), values...)
	})
	input := []int{1, 2}
	created, _ := memory.Create(ctx, "s", input)
	input[0] = 99
	created.Model[1] = 99
	loaded, _ := memory.Load(ctx, "s")
	if loaded.Model[0] != 1 || loaded.Model[1] != 2 {
		t.Fatalf("stored model mutated through caller: %v", loaded.Model)
	}
	loaded.Model[0] = 42
	again, _ := memory.Load(ctx, "s")
	if again.Model[0] != 1 {
		t.Fatalf("loaded copy shares memory: %v", again.Model)
	}
}
