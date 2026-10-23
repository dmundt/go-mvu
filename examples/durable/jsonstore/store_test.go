package jsonstore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dmundt/go-mvu/store"
)

type testModel struct {
	Count int
	Tags  map[string]string
}

func TestRestartIsolationAndBinaryKeys(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "models")
	key := string([]byte{0xff, 0x80, 0x01})
	first, err := Open[testModel](dir)
	if err != nil {
		t.Fatal(err)
	}
	input := testModel{Count: 2, Tags: map[string]string{"name": "original"}}
	created, err := first.Create(ctx, key, input)
	if err != nil || created.Version != 1 {
		t.Fatalf("create: %+v %v", created, err)
	}
	input.Tags["name"] = "caller"
	created.Model.Tags["name"] = "result"
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := Open[testModel](dir)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	loaded, err := restarted.Load(ctx, key)
	if err != nil || loaded.Model.Count != 2 || loaded.Model.Tags["name"] != "original" {
		t.Fatalf("load after restart: %+v %v", loaded, err)
	}
	keys, err := restarted.Keys()
	if err != nil || len(keys) != 1 || keys[0] != key {
		t.Fatalf("keys: %q %v", keys, err)
	}
}

func TestStoreContractAndConcurrentCAS(t *testing.T) {
	ctx := context.Background()
	models, err := Open[int](t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer models.Close()
	if _, err := models.Load(ctx, "k"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := models.CompareAndSwap(ctx, "k", 9, 1); !errors.Is(err, store.ErrConflict) {
		t.Fatal(err)
	}
	if _, err := models.CompareAndSwap(ctx, "k", 0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := models.Create(ctx, "k", 2); !errors.Is(err, store.ErrAlreadyExists) {
		t.Fatal(err)
	}

	var wins int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := models.CompareAndSwap(ctx, "k", 1, 3)
			if err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			} else if !errors.Is(err, store.ErrConflict) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("CAS winners = %d, want 1", wins)
	}
	if err := models.Delete(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := models.Delete(ctx, "missing"); err != nil {
		t.Fatal(err)
	}
	if _, err := models.Load(ctx, "k"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestInvalidJSONAndClosed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	models, err := Open[int](dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := models.Create(ctx, "k", 1); err != nil {
		t.Fatal(err)
	}
	name, _ := fileName("k")
	if err := os.WriteFile(filepath.Join(dir, name), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := models.Load(ctx, "k"); err == nil {
		t.Fatal("corrupt record accepted")
	}
	if err := models.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := models.Load(ctx, "k"); !errors.Is(err, ErrClosed) {
		t.Fatalf("load after close: %v", err)
	}
}

func TestSerializationFailureDoesNotPersist(t *testing.T) {
	models, err := Open[chan int](t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer models.Close()
	if _, err := models.Create(context.Background(), "bad", make(chan int)); err == nil {
		t.Fatal("unsupported JSON accepted")
	}
	if _, err := models.Load(context.Background(), "bad"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("failed write persisted: %v", err)
	}
}

func TestSchemaVersionValidation(t *testing.T) {
	dir := t.TempDir()
	models, err := Open[int](dir)
	if err != nil {
		t.Fatal(err)
	}
	defer models.Close()
	name, _ := fileName("k")
	data, err := json.Marshal(record[int]{Schema: 99, Version: 1, Model: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := models.Load(context.Background(), "k"); !errors.Is(err, ErrSchema) {
		t.Fatalf("schema error = %v", err)
	}
}
