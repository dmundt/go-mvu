package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testPath(t *testing.T) string {
	t.Helper()
	// Unique per test invocation, including concurrent Windows/WSL processes.
	// Keep fixtures inside this example rather than system temporary directories.
	dir, err := os.MkdirTemp(".", ".test-ledger-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return filepath.Join(dir, "ledger.wal")
}

func openTest(t *testing.T, path string) *Ledger {
	t.Helper()
	ledger, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ledger.Close(); err != nil {
			t.Error(err)
		}
	})
	return ledger
}

func closeTest(t *testing.T, ledger *Ledger) {
	t.Helper()
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
}

func applyTest(t *testing.T, ledger *Ledger, scope, id string, amount int64) Model {
	t.Helper()
	model, duplicate, err := ledger.Apply(context.Background(), scope, id, Add{Amount: amount})
	if err != nil || duplicate {
		t.Fatalf("Apply: model=%v duplicate=%t err=%v", model, duplicate, err)
	}
	return model
}

func pendingTest(t *testing.T, ledger *Ledger, count int) []Effect {
	t.Helper()
	effects, err := ledger.Pending()
	if err != nil || len(effects) != count {
		t.Fatalf("Pending: %v, %v; want %d", effects, err, count)
	}
	return effects
}

func TestRestartDedupAndScopes(t *testing.T) {
	path := testPath(t)
	ledger := openTest(t, path)
	applyTest(t, ledger, "alice", "first", 3)
	applyTest(t, ledger, "alice", "second", 2)
	applyTest(t, ledger, "bob", "first", 9)
	before := pendingTest(t, ledger, 3)
	if before[0].ID == before[2].ID {
		t.Fatal("effect ID did not include scope")
	}
	closeTest(t, ledger)
	ledger = openTest(t, path)
	after := pendingTest(t, ledger, 3)
	for i := range before {
		if before[i] != after[i] {
			t.Fatal("pending effect changed across restart")
		}
	}
	model, duplicate, err := ledger.Apply(
		context.Background(), "alice", "first", Add{Amount: 3},
	)
	if err != nil || !duplicate || model.Count != 3 {
		t.Fatalf("retry did not return original result: %v %t %v", model, duplicate, err)
	}
	if _, _, err := ledger.Apply(
		context.Background(), "alice", "first", Add{Amount: 4},
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting retry: %v", err)
	}
	model, err = ledger.Snapshot("alice")
	if err != nil || model.Count != 5 {
		t.Fatalf("snapshot: %v %v", model, err)
	}
	if err := ledger.DeliverPending(context.Background(),
		func(context.Context, Effect) error { return nil },
	); err != nil {
		t.Fatal(err)
	}
	closeTest(t, ledger)
	ledger = openTest(t, path)
	pendingTest(t, ledger, 0)
	if _, duplicate, err := ledger.Apply(
		context.Background(), "alice", "first", Add{Amount: 3},
	); err != nil || !duplicate {
		t.Fatalf("completed action dedup lost: %t %v", duplicate, err)
	}
}

func TestConcurrentDuplicates(t *testing.T) {
	ledger := openTest(t, testPath(t))
	var wg sync.WaitGroup
	var created atomic.Int32
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			model, duplicate, err := ledger.Apply(
				context.Background(), "scope", "same", Add{Amount: 7},
			)
			if err != nil || model.Count != 7 {
				t.Errorf("concurrent apply: %v %v", model, err)
			}
			if !duplicate {
				created.Add(1)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 || ledger.size != frameSize {
		t.Fatalf("created=%d size=%d", created.Load(), ledger.size)
	}
	pendingTest(t, ledger, 1)
}

func TestCapacityReservesCompletionAndNeverEvicts(t *testing.T) {
	path := testPath(t)
	ledger := openTest(t, path)
	for i := 0; i < maxEntries; i++ {
		applyTest(t, ledger, "scope", fmt.Sprint(i), 1)
	}
	pendingTest(t, ledger, maxEntries)
	if _, _, err := ledger.Apply(
		context.Background(), "scope", "overflow", Add{Amount: 1},
	); !errors.Is(err, ErrCapacity) {
		t.Fatalf("capacity: %v", err)
	}
	if err := ledger.DeliverPending(context.Background(),
		func(context.Context, Effect) error { return nil },
	); err != nil {
		t.Fatal(err)
	}
	if ledger.size != maxBytes {
		t.Fatalf("storage bound: %d != %d", ledger.size, maxBytes)
	}
	closeTest(t, ledger)
	ledger = openTest(t, path)
	pendingTest(t, ledger, 0)
	if _, duplicate, err := ledger.Apply(
		context.Background(), "scope", "0", Add{Amount: 1},
	); err != nil || !duplicate {
		t.Fatalf("capacity must still permit dedup: %t %v", duplicate, err)
	}
	if _, _, err := ledger.Apply(
		context.Background(), "scope", "overflow", Add{Amount: 1},
	); !errors.Is(err, ErrCapacity) {
		t.Fatalf("completed entries must not be evicted: %v", err)
	}
}

type faultyFile struct {
	ledgerFile
	writeLimit int
	writeErr   error
	syncErr    error
}

func (f *faultyFile) Write(data []byte) (int, error) {
	if f.writeLimit >= 0 {
		n, err := f.ledgerFile.Write(data[:f.writeLimit])
		return n, errors.Join(err, f.writeErr)
	}
	return f.ledgerFile.Write(data)
}

func (f *faultyFile) Sync() error {
	if f.syncErr != nil {
		return f.syncErr
	}
	return f.ledgerFile.Sync()
}

func TestWriteFailureBeforeMemoryCommit(t *testing.T) {
	for _, mode := range []string{"short", "write-error", "sync-error", "full-write-error"} {
		t.Run(mode, func(t *testing.T) {
			path := testPath(t)
			ledger := openTest(t, path)
			injected := errors.New("injected I/O error")
			fault := &faultyFile{ledgerFile: ledger.file, writeLimit: -1}
			switch mode {
			case "short":
				fault.writeLimit = 127
			case "write-error":
				fault.writeLimit = 127
				fault.writeErr = injected
			case "sync-error":
				fault.syncErr = injected
			case "full-write-error":
				fault.writeLimit = frameSize
				fault.writeErr = injected
			}
			ledger.file = fault
			if _, _, err := ledger.Apply(
				context.Background(), "scope", "id", Add{Amount: 1},
			); !errors.Is(err, ErrUncertain) {
				t.Fatalf("expected uncertain I/O failure: %v", err)
			}
			if len(ledger.entries) != 0 || len(ledger.models) != 0 || ledger.size != 0 {
				t.Fatal("failed persistence changed memory")
			}
			if _, err := ledger.Pending(); !errors.Is(err, ErrUncertain) {
				t.Fatalf("poisoned ledger allowed reads: %v", err)
			}
			if _, _, err := ledger.Apply(
				context.Background(), "scope", "other", Add{Amount: 1},
			); !errors.Is(err, ErrUncertain) {
				t.Fatalf("poisoned ledger allowed another write: %v", err)
			}
			closeTest(t, ledger)
			ledger = openTest(t, path)
			_, duplicate, err := ledger.Apply(
				context.Background(), "scope", "id", Add{Amount: 1},
			)
			wantDuplicate := mode == "sync-error" || mode == "full-write-error"
			if err != nil || duplicate != wantDuplicate {
				t.Fatalf("recovery duplicate=%t want=%t err=%v", duplicate, wantDuplicate, err)
			}
			model, err := ledger.Snapshot("scope")
			if err != nil || model.Count != 1 {
				t.Fatalf("recovery applied twice: %v %v", model, err)
			}
		})
	}
}

func TestEveryTornTailPreservesAcknowledgedPrefix(t *testing.T) {
	path := testPath(t)
	ledger := openTest(t, path)
	applyTest(t, ledger, "scope", "acknowledged", 2)
	closeTest(t, ledger)
	prefix, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	key := actionKey{Scope: "scope", ID: "unacknowledged"}
	tail, err := encodeFrame(actionRecord(key, Add{Amount: 3}, Model{Count: 5}))
	if err != nil {
		t.Fatal(err)
	}
	for length := 1; length < frameSize; length++ {
		data := append(append([]byte{}, prefix...), tail[:length]...)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		ledger, err := Open(path)
		if err != nil {
			t.Fatalf("tail length=%d: %v", length, err)
		}
		_, duplicate, err := ledger.Apply(
			context.Background(), "scope", "acknowledged", Add{Amount: 2},
		)
		if err != nil || !duplicate {
			t.Fatalf("lost acknowledged action at length %d: %t %v", length, duplicate, err)
		}
		model, err := ledger.Snapshot("scope")
		if err != nil || model.Count != 2 || ledger.size != frameSize {
			t.Fatalf("torn tail applied or prefix lost at length %d: %v %v", length, model, err)
		}
		pendingTest(t, ledger, 1)
		closeTest(t, ledger)
		info, err := os.Stat(path)
		if err != nil || info.Size() != frameSize {
			t.Fatalf("tail not truncated at length %d: %v %v", length, info, err)
		}
	}
}

func TestCorruptionFailsClosedWithoutTruncation(t *testing.T) {
	path := testPath(t)
	ledger := openTest(t, path)
	applyTest(t, ledger, "scope", "id", 1)
	closeTest(t, ledger)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int{0, 8, 12, 200, checksumAt, frameSize - 1} {
		data := append([]byte{}, original...)
		data[offset] ^= 1
		data = append(data, 1, 2, 3)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if ledger, err := Open(path); !errors.Is(err, ErrCorrupt) {
			if ledger != nil {
				closeTest(t, ledger)
			}
			t.Fatalf("corrupt offset %d: %v", offset, err)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(after) != string(data) {
			t.Fatalf("corrupt WAL was changed: %v", err)
		}
	}
	invalid, err := encodeFrame(record{
		Kind: "done",
		Key:  actionKey{Scope: "scope", ID: "missing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if ledger, err := Open(path); !errors.Is(err, ErrCorrupt) {
		if ledger != nil {
			closeTest(t, ledger)
		}
		t.Fatalf("checksummed invalid state accepted: %v", err)
	}
}

func TestExclusiveOwnershipAndStaleLock(t *testing.T) {
	path := testPath(t)
	ledger := openTest(t, path)
	if other, err := Open(path); !errors.Is(err, ErrLocked) {
		if other != nil {
			closeTest(t, other)
		}
		t.Fatalf("second owner: %v", err)
	}
	closeTest(t, ledger)
	if err := os.WriteFile(path+".lock", []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("stale lock removed automatically: %v", err)
	}
	content, err := os.ReadFile(path + ".lock")
	if err != nil || string(content) != "stale" {
		t.Fatalf("existing ownership marker changed: %q %v", content, err)
	}
}

func TestFailedEffectsSurviveRestart(t *testing.T) {
	path := testPath(t)
	ledger := openTest(t, path)
	applyTest(t, ledger, "scope", "id", 1)
	want := pendingTest(t, ledger, 1)[0]
	failure := errors.New("consumer unavailable")
	if err := ledger.DeliverPending(context.Background(),
		func(context.Context, Effect) error { return failure },
	); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	closeTest(t, ledger)
	ledger = openTest(t, path)
	if got := pendingTest(t, ledger, 1)[0]; got != want {
		t.Fatal("failed effect lost or ID changed")
	}
}

func TestConsumerDedupAfterSuccessAndTornCompletion(t *testing.T) {
	path := testPath(t)
	ledger := openTest(t, path)
	applyTest(t, ledger, "scope", "id", 1)
	effectID := pendingTest(t, ledger, 1)[0].ID
	receipt := filepath.Join(filepath.Dir(path), "consumer-receipt")
	calls := 0
	mutations := 0
	// This receipt models a consumer's separate durable inbox transaction.
	// Read it fresh on every delivery, including after reopening the ledger.
	consume := func(_ context.Context, effect Effect) (err error) {
		calls++
		if effect.ID != effectID {
			return errors.New("unstable effect ID")
		}
		data, err := os.ReadFile(receipt)
		if err == nil {
			if string(data) != effect.ID {
				return errors.New("consumer fingerprint conflict")
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		file, err := os.OpenFile(receipt, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, file.Close()) }()
		if _, err := io.WriteString(file, effect.ID); err != nil {
			return err
		}
		if err := file.Sync(); err != nil {
			return err
		}
		mutations++
		return nil
	}
	ledger.file = &faultyFile{ledgerFile: ledger.file, writeLimit: 50}
	if err := ledger.DeliverPending(context.Background(), consume); !errors.Is(err, ErrUncertain) {
		t.Fatalf("completion write: %v", err)
	}
	if ledger.entries[actionKey{Scope: "scope", ID: "id"}].Done {
		t.Fatal("completion committed before durable write")
	}
	closeTest(t, ledger)
	ledger = openTest(t, path)
	pendingTest(t, ledger, 1)
	if err := ledger.DeliverPending(context.Background(), consume); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || mutations != 1 {
		t.Fatalf("calls=%d mutations=%d; want 2,1", calls, mutations)
	}
	closeTest(t, ledger)
	pendingTest(t, openTest(t, path), 0)
}

func TestCompletionSyncFailure(t *testing.T) {
	path := testPath(t)
	ledger := openTest(t, path)
	applyTest(t, ledger, "scope", "id", 1)
	ledger.file = &faultyFile{
		ledgerFile: ledger.file,
		writeLimit: -1,
		syncErr:    errors.New("completion sync failed"),
	}
	calls := 0
	if err := ledger.DeliverPending(context.Background(),
		func(context.Context, Effect) error {
			calls++
			return nil
		},
	); !errors.Is(err, ErrUncertain) {
		t.Fatalf("completion sync: %v", err)
	}
	if calls != 1 || ledger.entries[actionKey{Scope: "scope", ID: "id"}].Done {
		t.Fatal("completion committed before successful sync")
	}
	closeTest(t, ledger)
	ledger = openTest(t, path)
	// A full completion frame survived; Open syncs and adopts it. The caller
	// must tolerate either this outcome or a still-pending effect after I/O failure.
	pendingTest(t, ledger, 0)
	model, duplicate, err := ledger.Apply(
		context.Background(), "scope", "id", Add{Amount: 1},
	)
	if err != nil || !duplicate || model.Count != 1 {
		t.Fatalf("completion failure lost action receipt: %v %t %v", model, duplicate, err)
	}
}

func TestRunnerRetryAndCancellation(t *testing.T) {
	ledger := openTest(t, testPath(t))
	applyTest(t, ledger, "scope", "id", 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	reports := 0
	err := ledger.Run(ctx, time.Millisecond, func(ctx context.Context, effect Effect) error {
		calls++
		if calls == 1 {
			return errors.New("transient")
		}
		cancel()
		return nil
	}, func(error) { reports++ })
	if !errors.Is(err, context.Canceled) || calls != 2 || reports != 1 {
		t.Fatalf("runner: calls=%d reports=%d err=%v", calls, reports, err)
	}
	pendingTest(t, ledger, 0)
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if _, _, err := ledger.Apply(ctx, "scope", "cancelled", Add{Amount: 1}); err == nil {
		t.Fatal("cancelled action applied")
	}
}

func TestInputValidationAndOversizeFile(t *testing.T) {
	path := testPath(t)
	ledger := openTest(t, path)
	for _, input := range []struct {
		scope  string
		id     string
		amount int64
	}{
		{"", "id", 1},
		{"scope", "", 1},
		{"scope", strings.Repeat("x", 65), 1},
		{"scope", "\xff", 1},
		{"scope", "id", 0},
		{"scope", "id", 1_000_001},
	} {
		if _, _, err := ledger.Apply(
			context.Background(), input.scope, input.id, Add{Amount: input.amount},
		); err == nil {
			t.Fatalf("accepted invalid input: %v", input)
		}
	}
	if ledger.size != 0 {
		t.Fatal("validation wrote WAL")
	}
	closeTest(t, ledger)
	if err := os.Truncate(path, maxBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); !errors.Is(err, ErrCapacity) {
		t.Fatalf("oversize file: %v", err)
	}
}
