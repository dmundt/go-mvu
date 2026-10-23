package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/session"
)

func TestOperationalRejectionCounters(t *testing.T) {
	diagnostics := NewDiagnostics(DiagnosticsOptions{})
	rate, err := RateLimit(RateLimitOptions{
		MaxEntries: 1, Requests: 1, Window: time.Minute,
		Key: func(r *http.Request) (string, error) { return r.Header.Get("Test-Key"), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := diagnostics.Middleware(rate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	})))
	for _, test := range []struct {
		key    string
		status int
	}{{"one", 204}, {"one", 429}, {"two", 503}} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Test-Key", test.key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("%s status %d", test.key, w.Code)
		}
	}
	options := phase4IdempotencyOptions()
	options.MaxEntries = 1
	record, err := Idempotency(options)
	if err != nil {
		t.Fatal(err)
	}
	handler = diagnostics.Middleware(record(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	})))
	handler.ServeHTTP(httptest.NewRecorder(), phase4ActionRequest("one", "{}"))
	handler.ServeHTTP(httptest.NewRecorder(), phase4ActionRequest("two", "{}"))
	counters := diagnostics.Snapshot().Counters
	for _, name := range []string{"rate_limited", "rate_capacity", "idempotency_capacity"} {
		if counters[name] != 1 {
			t.Fatalf("%s = %d", name, counters[name])
		}
	}
	if diagnostics.RecordEvent(httpx.Event(255)) {
		t.Fatal("unbounded label accepted")
	}
	diagnostics.Reset()
	if len(diagnostics.Snapshot().Counters) != 0 {
		t.Fatal("reset retained counters")
	}
}

func TestAdmissionCounterSurvivesConcurrentCollection(t *testing.T) {
	diagnostics := NewDiagnostics(DiagnosticsOptions{})
	admit, err := AdmissionLimit(AdmissionOptions{MaxConcurrent: 1, RetryAfter: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	handler := diagnostics.Middleware(admit(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(entered)
		<-release
	})))
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	<-entered
	var group sync.WaitGroup
	for range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
			_ = diagnostics.Snapshot()
		}()
	}
	group.Wait()
	close(release)
	<-done
	if got := diagnostics.Snapshot().Counters["admission_overload"]; got != 20 {
		t.Fatalf("overload count = %d", got)
	}
}

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

func TestEventObserverComposition(t *testing.T) {
	var first, second int
	ctx := httpx.WithEventObserver(context.Background(), func(httpx.Event) { first++ })
	ctx = httpx.WithEventObserver(ctx, func(httpx.Event) { second++ })
	httpx.Observe(ctx, httpx.StaleAction)
	if first != 1 || second != 1 {
		t.Fatal("nested collectors lost event")
	}
}
