package middleware

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/httpx"
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

func TestRequestPolicyBoundsHeaderCSRFBodyAndContentTypes(t *testing.T) {
	policy, err := NewRequestPolicy(RequestPolicyOptions{
		MaxBodyBytes: 4, ContentTypes: []string{"application/x-www-form-urlencoded"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		body, content string
		chunked       bool
		status        int
	}{
		{"12345", "application/x-www-form-urlencoded", false, 413},
		{"12345", "application/x-www-form-urlencoded", true, 413},
		{"ok", "application/json", false, 415},
		{"1234", "application/x-www-form-urlencoded; charset=utf-8", true, 204},
	} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(test.body))
		r.Header.Set("Content-Type", test.content)
		r.Header.Set(CSRFHeaderName, "header-token")
		if test.chunked {
			r.ContentLength = -1
		}
		w := httptest.NewRecorder()
		policy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := io.ReadAll(r.Body); err != nil {
				var tooLarge *http.MaxBytesError
				if !errors.As(err, &tooLarge) {
					t.Fatal(err)
				}
				httpx.RespondError(w, r, &httpx.Error{Status: 413, Cause: err})
				return
			}
			w.WriteHeader(204)
		})).ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("%+v: status %d", test, w.Code)
		}
	}
}

func TestRequestPolicyCooperativeDeadline(t *testing.T) {
	policy, err := NewRequestPolicy(RequestPolicyOptions{Timeout: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	policy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 504 {
		t.Fatalf("deadline status = %d", w.Code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w = httptest.NewRecorder()
	policy(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("canceled request reached handler")
	})).ServeHTTP(w, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	if w.Code != 408 {
		t.Fatalf("cancel status = %d", w.Code)
	}
}

func TestRequestPolicyRejectsInvalidConfiguration(t *testing.T) {
	for _, options := range []RequestPolicyOptions{
		{MaxBodyBytes: -1}, {Timeout: -1}, {ContentTypes: []string{"invalid"}},
		{ContentTypes: []string{"text/html; charset=utf-8"}},
		{ContentTypes: []string{"text/*"}},
	} {
		if _, err := NewRequestPolicy(options); err == nil {
			t.Fatalf("accepted %+v", options)
		}
	}
}
