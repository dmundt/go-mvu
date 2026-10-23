package timing

import (
	"context"
	"testing"
	"time"
)

func TestTraceSumsRepeatedPhasesInOrder(t *testing.T) {
	trace := New()
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
	trace := FromContext(context.Background())
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
	trace := New()
	ctx := WithTrace(context.Background(), trace)
	FromContext(ctx).Start(PhaseRender)()
	if phases := trace.Phases(); len(phases) != 1 || phases[0].Name != PhaseRender {
		t.Fatalf("phases = %v", phases)
	}
}

func BenchmarkStartWithoutTrace(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for range b.N {
		FromContext(ctx).Start(PhaseRender)()
	}
}
