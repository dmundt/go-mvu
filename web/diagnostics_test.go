package web

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func diagnosticsHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace := TraceFromContext(r.Context())
		trace.Add(PhaseDecode, time.Millisecond)
		trace.Add(PhaseView, 2*time.Millisecond)
		done := trace.Start(PhaseRender)
		w.WriteHeader(status)
		_, _ = w.Write([]byte("<p>ok</p>"))
		done()
	})
}

func TestDiagnosticsServerTimingHeaderAndTrailer(t *testing.T) {
	diagnostics := NewDiagnostics(DiagnosticsOptions{ServerTiming: true})
	recorder := httptest.NewRecorder()
	diagnostics.Middleware(diagnosticsHandler(http.StatusOK)).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	response := recorder.Result()
	header := response.Header.Get("Server-Timing")
	for _, metric := range []string{"decode;dur=1.000", "view;dur=2.000", "ttfb;dur="} {
		if !strings.Contains(header, metric) {
			t.Fatalf("Server-Timing header %q missing %q", header, metric)
		}
	}
	if strings.Contains(header, "render") {
		t.Fatalf("render must not appear before it finishes: %q", header)
	}
	trailer := response.Trailer.Get("Server-Timing")
	if !strings.Contains(trailer, "render;dur=") || !strings.Contains(trailer, "total;dur=") {
		t.Fatalf("Server-Timing trailer = %q", trailer)
	}
	if strings.Contains(trailer, "decode") {
		t.Fatalf("trailer repeats header phases: %q", trailer)
	}
}

func TestDiagnosticsServerTimingOffByDefault(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewDiagnostics(DiagnosticsOptions{}).Middleware(diagnosticsHandler(http.StatusOK)).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	response := recorder.Result()
	if response.Header.Get("Server-Timing") != "" || response.Trailer.Get("Server-Timing") != "" {
		t.Fatal("Server-Timing emitted without opt-in")
	}
}

func TestDiagnosticsAggregatesAndLogsSlowRequests(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	diagnostics := NewDiagnostics(DiagnosticsOptions{Logger: logger, SlowThreshold: time.Nanosecond})
	inner := diagnosticsHandler(http.StatusInternalServerError)
	handler := diagnostics.Middleware(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Millisecond)
			inner.ServeHTTP(w, r)
		}))
	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/users", nil)
		request = request.WithContext(WithRequestID(request.Context(), "test-request"))
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	snapshot := diagnostics.Snapshot()
	if snapshot.Requests != 2 || snapshot.Slow != 2 || snapshot.Errors != 2 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	decode := snapshot.Phases[PhaseDecode]
	if decode.Count != 2 || decode.TotalMS != 2 || decode.MeanMS != 1 || decode.MaxMS != 1 {
		t.Fatalf("decode stats = %+v", decode)
	}
	for _, name := range []string{PhaseRender, PhaseTTFB, PhaseTotal} {
		if snapshot.Phases[name].Count != 2 {
			t.Fatalf("%s count = %d", name, snapshot.Phases[name].Count)
		}
	}
	var record map[string]any
	if err := json.Unmarshal(bytes.SplitN(logs.Bytes(), []byte("\n"), 2)[0], &record); err != nil {
		t.Fatal(err)
	}
	if record["level"] != "WARN" || record["slow"] != true || record["request_id"] == "" {
		t.Fatalf("log record = %v", record)
	}
	if phases, ok := record["phases"].(map[string]any); !ok || phases["render"] == nil {
		t.Fatalf("log phases = %v", record["phases"])
	}
	diagnostics.Reset()
	if diagnostics.Snapshot().Requests != 0 {
		t.Fatal("Reset kept requests")
	}
}

func TestDiagnosticsHandlerServesJSON(t *testing.T) {
	diagnostics := NewDiagnostics(DiagnosticsOptions{})
	diagnostics.Middleware(diagnosticsHandler(http.StatusOK)).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	recorder := httptest.NewRecorder()
	diagnostics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/debug/diagnostics", nil))
	if recorder.Header().Get("Content-Type") != "application/json" ||
		recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %v", recorder.Header())
	}
	var snapshot DiagnosticsSnapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Requests != 1 || snapshot.Phases[PhaseView].Count != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	recorder = httptest.NewRecorder()
	diagnostics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/debug/diagnostics", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", recorder.Code)
	}
}

func TestMetricNameSanitizesTokens(t *testing.T) {
	if got := metricName(`db "query", x`); got != "db__query___x" {
		t.Fatalf("metricName = %q", got)
	}
	if got := metricName(""); got != "_" {
		t.Fatalf("empty metricName = %q", got)
	}
}
