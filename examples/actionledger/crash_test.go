package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Wrapping the real file gates production append boundaries without adding
// crash hooks to the ledger. The parent kills only its own child process.
type crashFile struct {
	ledgerFile
	kind string
}

func crashGate(stage string) {
	if os.Getenv("GOMVU_CRASH_STAGE") == stage {
		fmt.Println("crash-ready")
		time.Sleep(time.Hour)
		panic("crash parent did not terminate child")
	}
}

func (f *crashFile) Write(data []byte) (int, error) {
	rec, err := decodeFrame(data)
	if err != nil {
		return 0, err
	}
	f.kind = rec.Kind
	crashGate(f.kind + "-before-write")
	n, err := f.ledgerFile.Write(data)
	crashGate(f.kind + "-after-write")
	return n, err
}

func (f *crashFile) Sync() error {
	crashGate(f.kind + "-before-sync")
	err := f.ledgerFile.Sync()
	crashGate(f.kind + "-after-sync")
	return err
}

func TestLedgerCrashChild(t *testing.T) {
	path := os.Getenv("GOMVU_CRASH_PATH")
	if path == "" {
		t.Skip("subprocess fixture only")
	}
	ledger := openTest(t, path)
	inbox := openTest(t, path+".inbox")
	ledger.file = &crashFile{ledgerFile: ledger.file}
	applyTest(t, ledger, "scope", "action", 7)
	err := ledger.DeliverPending(context.Background(), func(ctx context.Context, effect Effect) error {
		crashGate("effect-before-commit")
		if _, _, err := inbox.Apply(ctx, effect.Scope, effect.ID, Add{Amount: 1}); err != nil {
			return err
		}
		crashGate("effect-after-commit")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Fatal("requested crash boundary was not reached")
}

func TestProcessCrashRecovery(t *testing.T) {
	stages := []string{
		"action-before-write", "action-after-write", "action-before-sync", "action-after-sync",
		"effect-before-commit", "effect-after-commit",
		"done-before-write", "done-after-write", "done-before-sync", "done-after-sync",
	}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			path, err := filepath.Abs(testPath(t))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLedgerCrashChild$", "-test.v")
			cmd.Env = append(os.Environ(), "GOMVU_CRASH_PATH="+path, "GOMVU_CRASH_STAGE="+stage)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			ready := make(chan bool, 1)
			go func() {
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					if scanner.Text() == "crash-ready" {
						ready <- true
						return
					}
				}
				ready <- false
			}()
			select {
			case ok := <-ready:
				if !ok {
					waitErr := cmd.Wait()
					t.Fatalf("child did not reach boundary: %v; %s", waitErr, stderr.String())
				}
			case <-ctx.Done():
				_ = cmd.Wait()
				t.Fatalf("child handshake: %v; %s", ctx.Err(), stderr.String())
			}
			killErr := cmd.Process.Kill()
			waitErr := cmd.Wait()
			if killErr != nil || waitErr == nil || ctx.Err() != nil {
				t.Fatalf("termination: kill=%v wait=%v context=%v", killErr, waitErr, ctx.Err())
			}
			// Only the parent that observed child exit may remove stale locks.
			for _, file := range []string{path, path + ".inbox"} {
				if err := os.Remove(file + ".lock"); err != nil {
					t.Fatal(err)
				}
			}
			ledger := openTest(t, path)
			inbox := openTest(t, path+".inbox")
			wantDuplicate := stage != "action-before-write"
			model, duplicate, err := ledger.Apply(context.Background(), "scope", "action", Add{Amount: 7})
			if err != nil || duplicate != wantDuplicate || model.Count != 7 {
				t.Fatalf("retry: model=%v duplicate=%t error=%v", model, duplicate, err)
			}
			snapshot, err := ledger.Snapshot("scope")
			if err != nil || snapshot.Count != 7 {
				t.Fatalf("snapshot=%v error=%v", snapshot, err)
			}
			effects := pendingTest(t, ledger, boolCount(!strings.HasPrefix(stage, "done-after") &&
				stage != "done-before-sync"))
			expectedID := digest(actionKey{Scope: "scope", ID: "action"})
			for _, effect := range effects {
				if effect.ID != expectedID {
					t.Fatalf("effect identity changed: %v", effect)
				}
			}
			calls := 0
			err = ledger.DeliverPending(
				context.Background(), func(ctx context.Context, effect Effect) error {
					calls++
					_, duplicate, err := inbox.Apply(
						ctx, effect.Scope, effect.ID, Add{Amount: 1},
					)
					wantDuplicate := stage == "effect-after-commit" || stage == "done-before-write"
					if err == nil && duplicate != wantDuplicate {
						t.Errorf("consumer duplicate=%t, want %t", duplicate, wantDuplicate)
					}
					return err
				})
			if err != nil || calls != len(effects) {
				t.Fatalf("delivery calls=%d error=%v", calls, err)
			}
			consumed, err := inbox.Snapshot("scope")
			if err != nil || consumed.Count != 1 {
				t.Fatalf("consumer applied effect more/less than once: %v %v", consumed, err)
			}
			pendingTest(t, ledger, 0)
			closeTest(t, ledger)
			ledger = openTest(t, path)
			pendingTest(t, ledger, 0)
		})
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
