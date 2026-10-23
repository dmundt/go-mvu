package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/web"
)

func TestRequestLimitsBoundsHeaderCSRFBody(t *testing.T) {
	for _, chunked := range []bool{false, true} {
		for _, size := range []int{(1 << 20) - 1, 1 << 20, (1 << 20) + 1} {
			r := httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", size)))
			r.Header.Set("X-CSRF-Token", "header-token")
			if chunked {
				r.ContentLength = -1
			}
			w := httptest.NewRecorder()
			requestLimits(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, err := io.ReadAll(r.Body); err != nil {
					var tooLarge *http.MaxBytesError
					if !errors.As(err, &tooLarge) {
						t.Fatal(err)
					}
					web.RespondError(w, r, &web.Error{Status: http.StatusRequestEntityTooLarge, Cause: err})
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}), time.Second).ServeHTTP(w, r)
			want := http.StatusNoContent
			if size > 1<<20 {
				want = http.StatusRequestEntityTooLarge
			}
			if w.Code != want {
				t.Fatalf("chunked=%t size=%d: status = %d, want %d", chunked, size, w.Code, want)
			}
		}
	}
}

func TestRequestLimitsCancellationAndCommitment(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	requestLimits(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("canceled request reached handler")
	}), time.Second).ServeHTTP(w, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	if w.Code != http.StatusRequestTimeout {
		t.Fatalf("canceled status = %d", w.Code)
	}
	for _, committed := range []bool{false, true} {
		w := httptest.NewRecorder()
		requestLimits(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if committed {
				w.WriteHeader(http.StatusNoContent)
			}
			<-r.Context().Done()
		}), time.Millisecond).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		want := http.StatusGatewayTimeout
		if committed {
			want = http.StatusNoContent
			if w.Body.Len() != 0 {
				t.Fatal("deadline appended an error after commitment")
			}
		}
		if w.Code != want {
			t.Fatalf("committed=%t: status = %d, want %d", committed, w.Code, want)
		}
	}
}
