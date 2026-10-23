package web

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestTraceSumsRepeatedPhasesInOrder(t *testing.T) {
	trace := NewTrace()
	trace.Add("load", time.Millisecond)
	trace.Add("render", 2*time.Millisecond)
	trace.Add("load", 3*time.Millisecond)
	got := trace.Phases()
	want := []Phase{{"load", 4 * time.Millisecond}, {"render", 2 * time.Millisecond}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("phases = %v, want %v", got, want)
	}
}

func TestNilTraceIsNoop(t *testing.T) {
	trace := TraceFromContext(context.Background())
	if trace != nil {
		t.Fatal("expected nil trace")
	}
	trace.Start("x")()
	trace.Add("x", time.Second)
	if trace.Phases() != nil || trace.Elapsed() != 0 {
		t.Fatal("nil trace recorded data")
	}
}

func TestStartRecordsThroughContext(t *testing.T) {
	trace := NewTrace()
	ctx := WithTrace(context.Background(), trace)
	TraceFromContext(ctx).Start(PhaseRender)()
	if phases := trace.Phases(); len(phases) != 1 || phases[0].Name != PhaseRender {
		t.Fatalf("phases = %v", phases)
	}
}

func TestTraceConcurrentCollectionAndSnapshotIsolation(t *testing.T) {
	trace := NewTrace()
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 100 {
				trace.Add(PhaseLoad, time.Millisecond)
				_ = trace.Phases()
			}
		}()
	}
	workers.Wait()
	phases := trace.Phases()
	if len(phases) != 1 || phases[0].Duration != 800*time.Millisecond {
		t.Fatalf("phases = %v, want one 800ms load phase", phases)
	}
	phases[0].Name = "changed"
	phases[0].Duration = 0
	if got := trace.Phases()[0]; got.Name != PhaseLoad || got.Duration != 800*time.Millisecond {
		t.Fatalf("snapshot mutation changed trace: %v", got)
	}
}

func BenchmarkStartWithoutTrace(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for range b.N {
		TraceFromContext(ctx).Start(PhaseRender)()
	}
}
