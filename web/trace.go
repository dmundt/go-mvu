package web

import (
	"context"
	"sync"
	"time"
)

// Phases recorded by the web runtime.
const (
	PhaseDecode   = "decode"
	PhaseDispatch = "dispatch"
	PhaseLoad     = "load"
	PhaseView     = "view"
	PhaseRender   = "render"
)

// Phase is one completed measurement.
type Phase struct {
	Name     string
	Duration time.Duration
}

// Trace collects phases for one request. Safe for concurrent use; methods are
// nil-safe so instrumentation requires no collector checks.
type Trace struct {
	mu      sync.Mutex
	started time.Time
	phases  []Phase
}

// NewTrace returns a trace starting now.
func NewTrace() *Trace { return &Trace{started: time.Now(), phases: make([]Phase, 0, 6)} }

type traceKey struct{}

// WithTrace attaches trace to ctx.
func WithTrace(ctx context.Context, trace *Trace) context.Context {
	return context.WithValue(ctx, traceKey{}, trace)
}

// TraceFromContext returns the attached trace or nil.
func TraceFromContext(ctx context.Context) *Trace {
	trace, _ := ctx.Value(traceKey{}).(*Trace)
	return trace
}

// Start begins phase name; call the returned func to record it.
func (t *Trace) Start(name string) func() {
	if t == nil {
		return func() {}
	}
	started := time.Now()
	return func() { t.Add(name, time.Since(started)) }
}

// Add records a completed phase. Repeated names are summed.
func (t *Trace) Add(name string, duration time.Duration) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.phases {
		if t.phases[i].Name == name {
			t.phases[i].Duration += duration
			return
		}
	}
	t.phases = append(t.phases, Phase{Name: name, Duration: duration})
}

// Phases returns a copy of recorded phases in first-seen order.
func (t *Trace) Phases() []Phase {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Phase(nil), t.phases...)
}

// Elapsed returns time since the trace started.
func (t *Trace) Elapsed() time.Duration {
	if t == nil {
		return 0
	}
	return time.Since(t.started)
}
