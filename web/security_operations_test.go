package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRevocationCapacityIsObservable(t *testing.T) {
	diagnostics := NewDiagnostics(DiagnosticsOptions{})
	backend, err := NewMemoryRevocations(1)
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := NewSession([]byte(strings.Repeat("k", 32)), SessionOptions{Revocations: backend})
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
				RespondError(w, r, err)
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
	if diagnostics.RecordEvent(Event(255)) {
		t.Fatal("unknown event accepted")
	}
}

func TestEventObserverComposition(t *testing.T) {
	var first, second int
	ctx := WithEventObserver(context.Background(), func(Event) { first++ })
	ctx = WithEventObserver(ctx, func(Event) { second++ })
	Observe(ctx, StaleAction)
	if first != 1 || second != 1 {
		t.Fatal("nested collectors lost event")
	}
}
