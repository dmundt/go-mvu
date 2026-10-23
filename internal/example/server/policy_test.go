package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

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
		{"123", "application/x-www-form-urlencoded", false, 204},
		{"1234", "application/x-www-form-urlencoded", false, 204},
		{"12345", "application/x-www-form-urlencoded", false, 413},
		{"123", "application/x-www-form-urlencoded", true, 204},
		{"12345", "application/x-www-form-urlencoded", true, 413},
		{"ok", "application/json", false, 415},
		{"1234", "application/x-www-form-urlencoded; charset=utf-8", true, 204},
	} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(test.body))
		r.Header.Set("Content-Type", test.content)
		r.Header.Set("X-CSRF-Token", "header-token")
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

func TestRequestPolicyDefaultBodyLimit(t *testing.T) {
	policy, err := NewRequestPolicy(RequestPolicyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{1 << 20, (1 << 20) + 1} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", size)))
		w := httptest.NewRecorder()
		policy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := io.ReadAll(r.Body); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(w, r)
		want := http.StatusNoContent
		if size > 1<<20 {
			want = http.StatusRequestEntityTooLarge
		}
		if w.Code != want {
			t.Fatalf("size=%d: status=%d, want %d", size, w.Code, want)
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

func TestRequestPolicyPreservesStreamingAndCommittedResponse(t *testing.T) {
	policy, err := NewRequestPolicy(RequestPolicyOptions{Timeout: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	policy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		if _, err := io.WriteString(w, "stream"); err != nil {
			t.Fatal(err)
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatal(err)
		}
		<-r.Context().Done()
	})).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if !w.Flushed || w.Code != http.StatusCreated || w.Body.String() != "stream" {
		t.Fatalf("flushed=%t status=%d body=%q", w.Flushed, w.Code, w.Body.String())
	}
}

func TestNoStorePreservesHandlerCacheOverride(t *testing.T) {
	for _, override := range []string{"", "private, max-age=60"} {
		w := httptest.NewRecorder()
		NoStore(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if override != "" {
				w.Header().Set("Cache-Control", override)
			}
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		want := override
		if want == "" {
			want = "no-store"
		}
		if got := w.Header().Get("Cache-Control"); got != want {
			t.Fatalf("Cache-Control = %q, want %q", got, want)
		}
	}
}
