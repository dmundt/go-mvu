package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"strings"
	"testing"
)

func TestRecoveryCompletesDiagnostics(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	diagnostics := NewDiagnostics(DiagnosticsOptions{Logger: logger})
	handler := diagnostics.Middleware(SecurityHeaders(Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret panic value")
	}))))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "secret") ||
		w.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("panic response = %d %s", w.Code, w.Body.String())
	}
	if diagnostics.Snapshot().Errors != 1 || diagnostics.Snapshot().Requests != 1 {
		t.Fatal("recovered panic missing from diagnostics")
	}
	if !strings.Contains(logs.String(), `"msg":"request diagnostics"`) {
		t.Fatalf("completion diagnostics missing: %s", logs.String())
	}
}

func TestTrackingMiddlewareInformationalFlushAndTrailer(t *testing.T) {
	diagnostics := NewDiagnostics(DiagnosticsOptions{ServerTiming: true})
	handler := diagnostics.Middleware(Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Trailer", "X-Complete")
		w.WriteHeader(http.StatusEarlyHints)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "stream")
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
		}
		w.Header().Set("X-Complete", "done")
	})))
	server := httptest.NewServer(handler)
	defer server.Close()
	hints := make(chan int, 1)
	r, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	r = r.WithContext(httptrace.WithClientTrace(r.Context(), &httptrace.ClientTrace{
		Got1xxResponse: func(status int, _ textproto.MIMEHeader) error {
			hints <- status
			return nil
		},
	}))
	response, err := server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 201 || string(body) != "stream" ||
		response.Trailer.Get("X-Complete") != "done" ||
		!strings.Contains(response.Trailer.Get("Server-Timing"), "total;dur=") {
		t.Fatalf("response = %d %s, trailers %v", response.StatusCode, body, response.Trailer)
	}
	select {
	case status := <-hints:
		if status != 103 {
			t.Fatalf("informational status = %d", status)
		}
	default:
		t.Fatal("informational response lost")
	}
}

func TestPanicAfterFlushAbortsWithoutAppendingError(t *testing.T) {
	for _, sentinel := range []bool{false, true} {
		w := httptest.NewRecorder()
		diagnostics := NewDiagnostics(DiagnosticsOptions{})
		handler := diagnostics.Middleware(Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "partial")
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Fatal(err)
			}
			if sentinel {
				panic(http.ErrAbortHandler)
			}
			panic("late panic")
		})))
		func() {
			defer func() {
				if got := recover(); got != http.ErrAbortHandler {
					t.Fatalf("panic = %v, want ErrAbortHandler", got)
				}
			}()
			handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		}()
		if !w.Flushed || w.Body.String() != "partial" || diagnostics.Snapshot().Requests != 1 {
			t.Fatalf("late panic response = %s, diagnostics %+v", w.Body.String(), diagnostics.Snapshot())
		}
	}
}
