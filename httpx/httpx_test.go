package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type headerWriter struct {
	*httptest.ResponseRecorder
	statuses []int
}

func (w *headerWriter) WriteHeader(status int) {
	w.statuses = append(w.statuses, status)
	if status >= 200 {
		w.ResponseRecorder.WriteHeader(status)
	}
}

func TestWriterInformationalAndFinalHeaders(t *testing.T) {
	base := &headerWriter{ResponseRecorder: httptest.NewRecorder()}
	w := &Writer{ResponseWriter: base}
	w.WriteHeader(http.StatusEarlyHints)
	if w.Started {
		t.Fatal("informational response committed final status")
	}
	w.WriteHeader(http.StatusCreated)
	w.WriteHeader(http.StatusInternalServerError)
	if _, err := w.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if w.Status != 201 || w.Bytes != 3 || len(base.statuses) != 2 {
		t.Fatalf("writer = %+v, statuses = %v", w, base.statuses)
	}
}

func TestWriterControllerFlushCommitsEachLayer(t *testing.T) {
	base := httptest.NewRecorder()
	first := &Writer{ResponseWriter: base}
	second := &Writer{ResponseWriter: first}
	if err := http.NewResponseController(second).Flush(); err != nil {
		t.Fatal(err)
	}
	if !base.Flushed || !first.Started || !second.Started || second.Status != 200 {
		t.Fatal("flush bypassed response tracking")
	}
}

type deadlineWriter struct {
	*httptest.ResponseRecorder
	read, write time.Time
	duplex      bool
}

func (w *deadlineWriter) SetReadDeadline(deadline time.Time) error {
	w.read = deadline
	return nil
}

func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.write = deadline
	return nil
}

func (w *deadlineWriter) EnableFullDuplex() error {
	w.duplex = true
	return nil
}

func TestWriterControllerPreservesCapabilities(t *testing.T) {
	base := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	tracked := &Writer{ResponseWriter: &Writer{ResponseWriter: base}}
	controller := http.NewResponseController(tracked)
	deadline := time.Now().Add(time.Minute)
	if err := controller.SetReadDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := controller.SetWriteDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := controller.EnableFullDuplex(); err != nil {
		t.Fatal(err)
	}
	if base.read != deadline || base.write != deadline || !base.duplex || tracked.Started {
		t.Fatal("controller failed to forward capabilities")
	}
	if _, _, err := controller.Hijack(); !errors.Is(err, http.ErrNotSupported) || tracked.Started {
		t.Fatalf("unsupported hijack: %v, started %v", err, tracked.Started)
	}
}

func TestVaryMergesCaseInsensitively(t *testing.T) {
	header := make(http.Header)
	header.Add("Vary", "Accept-Encoding, HX-Request")
	Vary(header, "hx-request", "HX-Boosted")
	if len(header.Values("Vary")) != 2 {
		t.Fatalf("Vary = %v", header.Values("Vary"))
	}
	header.Set("Vary", "*")
	Vary(header, "Cookie")
	if header.Get("Vary") != "*" || len(header.Values("Vary")) != 1 {
		t.Fatal("wildcard changed")
	}
}

func TestRespondErrorStatusPrivacyAndNavigation(t *testing.T) {
	for _, test := range []struct {
		name     string
		err      error
		htmx     bool
		status   int
		location string
	}{
		{"internal", errors.New("private database password"), false, 500, ""},
		{"deadline", context.DeadlineExceeded, false, 504, ""},
		{"cancel", context.Canceled, false, 408, ""},
		{"validation", &Error{Status: 422, Public: "Invalid email"}, true, 422, ""},
		{"login", &Error{Status: 401, Location: "/login"}, false, 303, "/login"},
		{"htmx login", &Error{Status: 401, Location: "/login"}, true, 401, "/login"},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r = r.WithContext(WithRequestID(r.Context(), "test-id"))
			if test.htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			RespondError(w, r, test.err)
			if w.Code != test.status || strings.Contains(w.Body.String(), "password") {
				t.Fatalf("response: %d %s", w.Code, w.Body.String())
			}
			header := "Location"
			if test.htmx {
				header = "HX-Redirect"
			}
			if w.Header().Get(header) != test.location {
				t.Fatalf("%s = %q", header, w.Header().Get(header))
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("error cacheable")
			}
		})
	}
}
