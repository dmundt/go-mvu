package middleware

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

// Pseudo-phases added by Diagnostics next to runtime phases.
const (
	PhaseTTFB  = "ttfb"
	PhaseTotal = "total"
)

// DiagnosticsOptions configures Diagnostics.
type DiagnosticsOptions struct {
	// Logger receives one record per request: Debug normally, Warn when
	// SlowThreshold is exceeded. nil uses slog.Default().
	Logger *slog.Logger
	// SlowThreshold > 0 marks slower requests as slow.
	SlowThreshold time.Duration
	// ServerTiming emits a Server-Timing header with phases finished before
	// the first byte, plus a Server-Timing trailer with render and total.
	// Exposes server timings to clients; enable for development or trusted
	// networks.
	ServerTiming bool
}

// PhaseStats aggregates one phase across requests.
type PhaseStats struct {
	Count   uint64  `json:"count"`
	TotalMS float64 `json:"total_ms"`
	MeanMS  float64 `json:"mean_ms"`
	MaxMS   float64 `json:"max_ms"`
}

// DiagnosticsSnapshot is a point-in-time copy of aggregated diagnostics.
type DiagnosticsSnapshot struct {
	Requests uint64                `json:"requests"`
	Slow     uint64                `json:"slow"`
	Errors   uint64                `json:"errors"`
	Aborted  uint64                `json:"aborted"`
	Phases   map[string]PhaseStats `json:"phases"`
	Counters map[string]uint64     `json:"counters"`
}

type phaseTotals struct {
	count uint64
	total time.Duration
	max   time.Duration
}

// Diagnostics measures request phases (decode, dispatch, load, view,
// render, recorded by the web runtime through package httpx), time to
// first byte, and total time. It logs per-request records and keeps
// in-memory aggregates. Safe for concurrent use.
type Diagnostics struct {
	logger       *slog.Logger
	slow         time.Duration
	serverTiming bool

	mu       sync.Mutex
	requests uint64
	slowN    uint64
	errors   uint64
	aborted  uint64
	phases   map[string]*phaseTotals
	counters map[httpx.Event]uint64
}

// NewDiagnostics returns a Diagnostics collector.
func NewDiagnostics(options DiagnosticsOptions) *Diagnostics {
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Diagnostics{
		logger:       logger,
		slow:         options.SlowThreshold,
		serverTiming: options.ServerTiming,
		phases:       map[string]*phaseTotals{},
		counters:     map[httpx.Event]uint64{},
	}
}

// Middleware attaches a httpx.Trace to each request and records results.
func (d *Diagnostics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace := httpx.NewTrace()
		writer := &timingWriter{ResponseWriter: w, trace: trace, serverTiming: d.serverTiming}
		completed := false
		defer func() {
			total := trace.Elapsed()
			if !writer.started {
				writer.ttfb = total
			}
			phases := trace.Phases()
			if d.serverTiming && writer.started {
				w.Header().Set(http.TrailerPrefix+"Server-Timing",
					serverTiming(after(phases, writer.sent), httpx.Phase{Name: PhaseTotal, Duration: total}))
			}
			status := writer.status
			if status == 0 {
				status = http.StatusOK
				if !completed {
					status = http.StatusInternalServerError
				}
			}
			d.record(r, status, writer.bytes, writer.ttfb, total, phases, !completed)
		}()
		ctx := httpx.WithEventObserver(httpx.WithTrace(r.Context(), trace), func(event httpx.Event) {
			d.RecordEvent(event)
		})
		next.ServeHTTP(writer, r.WithContext(ctx))
		completed = true
	})
}

func (d *Diagnostics) record(
	r *http.Request, status, bytes int, ttfb, total time.Duration, phases []httpx.Phase, aborted bool,
) {
	slow := d.slow > 0 && total > d.slow
	d.mu.Lock()
	d.requests++
	if slow {
		d.slowN++
	}
	if status >= http.StatusInternalServerError || aborted {
		d.errors++
	}
	if aborted {
		d.aborted++
		d.counters[httpx.ResponseAborted]++
	}
	for _, phase := range phases {
		d.add(phase.Name, phase.Duration)
	}
	d.add(PhaseTTFB, ttfb)
	d.add(PhaseTotal, total)
	d.mu.Unlock()

	level := slog.LevelDebug
	if slow {
		level = slog.LevelWarn
	}
	if !d.logger.Enabled(r.Context(), level) {
		return
	}
	attrs := make([]any, 0, len(phases))
	for _, phase := range phases {
		attrs = append(attrs, slog.Duration(phase.Name, phase.Duration))
	}
	d.logger.LogAttrs(r.Context(), level, "request diagnostics",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Int("status", status),
		slog.Int("bytes", bytes),
		slog.Duration("ttfb", ttfb),
		slog.Duration("total", total),
		slog.Bool("slow", slow),
		slog.Bool("aborted", aborted),
		slog.String("request_id", httpx.RequestID(r.Context())),
		slog.Group("phases", attrs...),
	)
}

// add requires d.mu.
func (d *Diagnostics) add(name string, duration time.Duration) {
	totals := d.phases[name]
	if totals == nil {
		totals = &phaseTotals{}
		d.phases[name] = totals
	}
	totals.count++
	totals.total += duration
	totals.max = max(totals.max, duration)
}

// Snapshot returns aggregated diagnostics.
func (d *Diagnostics) Snapshot() DiagnosticsSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	snapshot := DiagnosticsSnapshot{
		Requests: d.requests, Slow: d.slowN, Errors: d.errors, Aborted: d.aborted,
		Phases:   make(map[string]PhaseStats, len(d.phases)),
		Counters: make(map[string]uint64, len(d.counters)),
	}
	for name, totals := range d.phases {
		snapshot.Phases[name] = PhaseStats{
			Count:   totals.count,
			TotalMS: milliseconds(totals.total),
			MeanMS:  milliseconds(totals.total / time.Duration(totals.count)),
			MaxMS:   milliseconds(totals.max),
		}
	}
	for event, count := range d.counters {
		snapshot.Counters[event.String()] = count
	}
	return snapshot
}

// RecordEvent supports application-owned boundaries such as outbox workers.
// Unknown enum values are rejected without allocating a metric label.
func (d *Diagnostics) RecordEvent(event httpx.Event) bool {
	if event.String() == "" {
		return false
	}
	d.mu.Lock()
	d.counters[event]++
	d.mu.Unlock()
	return true
}

// Reset clears aggregates.
func (d *Diagnostics) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requests, d.slowN, d.errors = 0, 0, 0
	d.aborted = 0
	d.phases = map[string]*phaseTotals{}
	d.counters = map[httpx.Event]uint64{}
}

// Handler serves Snapshot as JSON. Mount on an internal or authenticated
// route only.
func (d *Diagnostics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodHead {
			return
		}
		if err := json.NewEncoder(w).Encode(d.Snapshot()); err != nil {
			d.logger.ErrorContext(r.Context(), "diagnostics response failed", "error", err)
		}
	})
}

// timingWriter emits Server-Timing for phases finished before the first
// byte and records status, bytes, and time to first byte.
type timingWriter struct {
	http.ResponseWriter
	trace        *httpx.Trace
	serverTiming bool
	started      bool
	sent         int
	status       int
	bytes        int
	ttfb         time.Duration
}

func (w *timingWriter) begin(status int) {
	if w.started {
		return
	}
	w.started = true
	w.status = status
	w.ttfb = w.trace.Elapsed()
	if w.serverTiming {
		phases := w.trace.Phases()
		w.sent = len(phases)
		w.Header().Set("Server-Timing",
			serverTiming(phases, httpx.Phase{Name: PhaseTTFB, Duration: w.ttfb}))
	}
}

func (w *timingWriter) WriteHeader(status int) {
	if w.started {
		return
	}
	if status < 100 || status > 999 {
		panic("invalid HTTP status code")
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.begin(status)
	w.ResponseWriter.WriteHeader(status)
}

func (w *timingWriter) Write(p []byte) (int, error) {
	w.begin(http.StatusOK)
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach Flush and deadlines.
func (w *timingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *timingWriter) FlushError() error {
	if !w.started {
		w.WriteHeader(http.StatusOK)
	}

	return http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *timingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, buffer, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.begin(http.StatusSwitchingProtocols)
	}
	return conn, buffer, err
}

// after returns phases first recorded after the first sent ones. Phases
// already in the header and extended later are not repeated.
func after(phases []httpx.Phase, sent int) []httpx.Phase {
	if sent >= len(phases) {
		return nil
	}
	return phases[sent:]
}

func serverTiming(phases []httpx.Phase, extra httpx.Phase) string {
	all := append(append([]httpx.Phase(nil), phases...), extra)
	parts := make([]string, 0, len(all))
	for _, phase := range all {
		parts = append(parts, metricName(phase.Name)+";dur="+
			strconv.FormatFloat(milliseconds(phase.Duration), 'f', 3, 64))
	}
	return strings.Join(parts, ", ")
}

// metricName keeps RFC 9110 token characters; others become '_'.
func metricName(name string) string {
	if name == "" {
		return "_"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case strings.ContainsRune("!#$%&'*+-.^_`|~", r):
			return r
		}
		return '_'
	}, name)
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}
