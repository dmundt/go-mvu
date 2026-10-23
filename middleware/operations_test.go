package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/session"
)

func TestRevocationCapacityIsObservable(t *testing.T) {
	diagnostics := NewDiagnostics(DiagnosticsOptions{})
	backend, err := session.NewMemoryRevocations(1)
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := session.New([]byte(strings.Repeat("k", 32)), session.Options{Revocations: backend})
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		get := httptest.NewRecorder()
		sessions.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
			ServeHTTP(get, httptest.NewRequest("GET", "/", nil))
		r := httptest.NewRequest("POST", "/logout", nil)
		r.AddCookie(get.Result().Cookies()[0])
		handler := diagnostics.Middleware(sessions.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := sessions.Invalidate(w, r); err != nil {
				httpx.RespondError(w, r, err)
				return
			}
			w.WriteHeader(204)
		})))
		handler.ServeHTTP(httptest.NewRecorder(), r)
	}
	if diagnostics.Snapshot().Counters["revocation_capacity"] != 1 {
		t.Fatal("revocation saturation missing")
	}
}

func TestEventObserverRejectsUnknownValues(t *testing.T) {
	diagnostics := NewDiagnostics(DiagnosticsOptions{})
	if diagnostics.RecordEvent(httpx.Event(255)) {
		t.Fatal("unknown event accepted")
	}
}

func TestEventObserverComposition(t *testing.T) {
	var first, second int
	ctx := httpx.WithEventObserver(context.Background(), func(httpx.Event) { first++ })
	ctx = httpx.WithEventObserver(ctx, func(httpx.Event) { second++ })
	httpx.Observe(ctx, httpx.StaleAction)
	if first != 1 || second != 1 {
		t.Fatal("nested collectors lost event")
	}
}
