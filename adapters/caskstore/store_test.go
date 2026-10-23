package caskstore

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dmundt/go-mvu/store"
)

type model struct {
	Count int
	Tags  map[string]string
}

var _ store.Store[string, model] = (*Store[model])(nil)

func openTest[M any](t *testing.T, dir string) *Store[M] {
	t.Helper()
	s, err := Open[M](dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func TestRestartAndIsolation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest[model](t, dir)
	input := model{Count: 2, Tags: map[string]string{"name": "original"}}
	first, err := s.Create(ctx, "../binary\x00key", input)
	if err != nil || first.Version != 1 {
		t.Fatalf("create: %+v %v", first, err)
	}
	input.Tags["name"] = "caller"
	first.Model.Tags["name"] = "result"
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTest[model](t, dir)
	loaded, err := s.Load(ctx, "../binary\x00key")
	if err != nil || loaded.Model.Count != 2 || loaded.Model.Tags["name"] != "original" {
		t.Fatalf("load after restart: %+v %v", loaded, err)
	}
	loaded.Model.Tags["name"] = "loaded"
	again, err := s.Load(ctx, "../binary\x00key")
	if err != nil || again.Model.Tags["name"] != "original" {
		t.Fatalf("load aliases: %+v %v", again, err)
	}
}

func TestStoreContractAndConcurrentCAS(t *testing.T) {
	ctx := context.Background()
	s := openTest[model](t, t.TempDir())
	if _, err := s.Load(ctx, "k"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.CompareAndSwap(ctx, "k", 9, model{}); !errors.Is(err, store.ErrConflict) {
		t.Fatal(err)
	}
	if _, err := s.CompareAndSwap(ctx, "k", 0, model{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "k", model{}); !errors.Is(err, store.ErrAlreadyExists) {
		t.Fatal(err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.CompareAndSwap(ctx, "k", 1, model{Count: 1})
			if err == nil {
				wins.Add(1)
			} else if !errors.Is(err, store.ErrConflict) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("CAS winners = %d", wins.Load())
	}
	if err := s.Delete(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "missing"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(ctx, "k"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "k", model{Count: 4}); err != nil {
		t.Fatal(err)
	}
}

func TestOwnershipCancellationAndClosed(t *testing.T) {
	dir := t.TempDir()
	s := openTest[model](t, dir)
	if other, err := Open[model](dir); !errors.Is(err, ErrLocked) {
		if other != nil {
			_ = other.Close()
		}

		t.Fatalf("second owner: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Create(ctx, "k", model{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(context.Background(), "k"); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	_ = openTest[model](t, dir)
}

func TestCrossProcessOwnership(t *testing.T) {
	if dir := os.Getenv("GOMVU_CASK_LOCK_TEST"); dir != "" {
		s, err := Open[model](dir)
		if !errors.Is(err, ErrLocked) {
			if s != nil {
				_ = s.Close()
			}
			t.Fatalf("child acquired owned directory: %v", err)
		}
		return
	}
	dir := t.TempDir()
	_ = openTest[model](t, dir)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestCrossProcessOwnership$")
	child.Env = append(os.Environ(), "GOMVU_CASK_LOCK_TEST="+dir)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("child: %v\n%s", err, output)
	}
}

func TestCorruptionAndSerializationFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest[model](t, dir)
	if _, err := s.Create(ctx, "k", model{Count: 5}); err != nil {
		t.Fatal(err)
	}
	name, err := keyName("k")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "heads", filepath.FromSlash(name)),
		[]byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(ctx, "k"); err == nil {
		t.Fatal("corrupt head accepted")
	}
	bad := openTest[chan int](t, t.TempDir())
	if _, err := bad.Create(ctx, "bad", make(chan int)); err == nil {
		t.Fatal("unsupported JSON accepted")
	}
	if _, err := bad.Load(ctx, "bad"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("failed write persisted: %v", err)
	}
}

func TestPublicationFailureReportsUncertainCommit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest[model](t, dir)
	if _, err := s.Create(ctx, "k", model{Count: 1}); err != nil {
		t.Fatal(err)
	}
	name, err := keyName("k")
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "heads", ".log", filepath.FromSlash(name))
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(log, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompareAndSwap(ctx, "k", 1, model{Count: 2}); !errors.Is(err, ErrCommitUncertain) {
		t.Fatalf("publication failure: %v", err)
	}
	current, err := s.Load(ctx, "k")
	if err != nil || current.Version != 2 || current.Model.Count != 2 {
		t.Fatalf("reconcile committed publication: %+v %v", current, err)
	}
}

func TestSchemaAndObjectFailures(t *testing.T) {
	ctx := context.Background()
	s := openTest[model](t, t.TempDir())
	name, _ := keyName("k")
	if _, err := s.write(ctx, name, record[model]{Schema: 99, Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(ctx, "k"); !errors.Is(err, ErrSchema) {
		t.Fatalf("schema: %v", err)
	}
	digest, err := s.heads.Get(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.backend.Delete(ctx, digest); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(ctx, "k"); err == nil || errors.Is(err, store.ErrNotFound) {
		t.Fatalf("dangling object must not initialize new model: %v", err)
	}
}
