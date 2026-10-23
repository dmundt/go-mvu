package middleware

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

func phase4IdempotencyOptions() IdempotencyOptions {
	return IdempotencyOptions{
		MaxEntries: 8, TTL: time.Minute, MaxBodyBytes: 1024, MaxResponseBytes: 4096, Route: "orders.create",
		Scope: func(r *http.Request) (string, error) { return r.Header.Get("Test-Principal"), nil },
	}
}

func phase4ActionRequest(action, body string) *http.Request {
	r := httptest.NewRequest("POST", "/orders", strings.NewReader(body))
	r.Header.Set("Idempotency-Key", action)
	r.Header.Set("Test-Principal", "principal-1")
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestPhase4IdempotencySingleFlight(t *testing.T) {
	options := phase4IdempotencyOptions()
	options.MaxWaiters = 64
	adapter, err := Idempotency(options)
	if err != nil {
		t.Fatal(err)
	}
	var count atomic.Int32
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count.Add(1) == 1 {
			close(entered)
		}
		<-release
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"x":1}` {
			t.Errorf("handler body %q", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":7}`)
	}))
	var leader *httptest.ResponseRecorder
	go func() {
		defer close(done)
		leader = httptest.NewRecorder()
		handler.ServeHTTP(leader, phase4ActionRequest("same", `{"x":1}`))
	}()
	<-entered
	conflict := httptest.NewRecorder()
	handler.ServeHTTP(conflict, phase4ActionRequest("same", `{"x":2}`))
	if conflict.Code != 409 {
		t.Fatal("conflicting in-flight action not rejected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceled := httptest.NewRecorder()
	handler.ServeHTTP(canceled, phase4ActionRequest("same", `{"x":1}`).WithContext(ctx))
	if canceled.Code != 408 {
		t.Fatalf("canceled request got %d", canceled.Code)
	}
	// Cancel a duplicate already waiting on the pending mutation.
	ctx, cancel = context.WithCancel(context.Background())
	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, phase4ActionRequest("same", `{"x":1}`).WithContext(ctx))
		if w.Code != 408 {
			t.Errorf("waiting cancel got %d", w.Code)
		}
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("duplicate ignored cancellation")
	}
	var group sync.WaitGroup
	for i := 0; i < 24; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, phase4ActionRequest("same", `{"x":1}`))
			if w.Code != 201 || w.Body.String() != `{"id":7}` {
				t.Errorf("duplicate %d %q", w.Code, w.Body.String())
			}
		}()
	}
	close(release)
	<-done
	group.Wait()
	if count.Load() != 1 || leader.Code != 201 {
		t.Fatalf("executions %d, leader %d", count.Load(), leader.Code)
	}
}

func TestPhase4IdempotencyReplayMetadata(t *testing.T) {
	options := phase4IdempotencyOptions()
	options.ReplayHeaders = []string{"X-Action-Result"}
	adapter, _ := Idempotency(options)
	var count int
	handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Set-Cookie", "session=old")
		w.Header().Set("Server-Timing", "old;dur=7")
		w.Header().Set("X-Request-ID", "first")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/orders/7")
		w.Header().Set("Cache-Control", "public")
		w.Header().Set("X-Action-Result", "created")
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"id":7}`)
	}))
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		w.Header().Set("X-Request-ID", fmt.Sprintf("current-%d", i))
		w.Header().Set("Content-Security-Policy", fmt.Sprintf("current-policy-%d", i))
		w.Header().Set("Server-Timing", "current")
		if i == 1 {
			w.Header().Set("Set-Cookie", "session=current")
		}
		handler.ServeHTTP(w, phase4ActionRequest("same", "{}"))
		if w.Code != 201 || w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Location") != "/orders/7" {
			t.Fatalf("replay status/representation %d %v", w.Code, w.Header())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("action response publicly cacheable")
		}
		if w.Header().Get("X-Action-Result") != "created" {
			t.Fatal("explicit application header not replayed")
		}
		if i == 1 && (w.Header().Get("Set-Cookie") != "session=current" ||
			w.Header().Get("Server-Timing") != "current" || w.Header().Get("X-Request-ID") != "current-1" ||
			w.Header().Get("Content-Security-Policy") != "current-policy-1") {
			t.Fatalf("replayed request metadata: %v", w.Header())
		}
	}
	if count != 1 {
		t.Fatal("replay reran handler")
	}
}

func TestPhase4IdempotencyFailureIsTerminal(t *testing.T) {
	for _, mode := range []string{"500", "overflow", "headers", "flush", "hijack", "trailer", "panic"} {
		t.Run(mode, func(t *testing.T) {
			options := phase4IdempotencyOptions()
			options.MaxResponseBytes = 256
			adapter, _ := Idempotency(options)
			var count int
			handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				switch mode {
				case "500":
					w.WriteHeader(500)
					_, _ = io.WriteString(w, "committed failure")
				case "overflow":
					_, err := io.WriteString(w, strings.Repeat("x", 257))
					if !errors.Is(err, ErrIdempotencyResponseLimit) {
						t.Errorf("overflow err %v", err)
					}
				case "headers":
					w.Header().Set("Too-Large", strings.Repeat("x", 257))
				case "flush":
					if err := http.NewResponseController(w).Flush(); !errors.Is(err, http.ErrNotSupported) {
						t.Errorf("flush err %v", err)
					}
				case "hijack":
					if _, _, err := http.NewResponseController(w).Hijack(); !errors.Is(err, http.ErrNotSupported) {
						t.Errorf("hijack err %v", err)
					}
				case "trailer":
					w.Header().Set("Trailer", "Digest")
					_, _ = io.WriteString(w, "data")
				case "panic":
					panic("mutation may be committed")
				}
			}))
			for i := 0; i < 2; i++ {
				w := httptest.NewRecorder()
				func() {
					defer func() {
						value := recover()
						if mode == "panic" && i == 0 && value == nil {
							t.Error("panic swallowed")
						}
						if value != nil && (mode != "panic" || i != 0) {
							t.Errorf("unexpected panic %v", value)
						}
					}()
					handler.ServeHTTP(w, phase4ActionRequest("same", "{}"))
				}()
				if !(mode == "panic" && i == 0) && w.Code != 500 {
					t.Fatalf("terminal failure got %d", w.Code)
				}
				if mode == "500" && w.Body.String() != "committed failure" {
					t.Fatal("completed failure response not replayed")
				}
			}
			if count != 1 {
				t.Fatalf("ambiguous failure reran mutation %d times", count)
			}
		})
	}
}

func TestPhase4IdempotencyCapacityExpiryAndScope(t *testing.T) {
	options := phase4IdempotencyOptions()
	options.MaxEntries, options.TTL = 1, time.Second
	adapter, _ := Idempotency(options)
	var count int
	handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count++; w.WriteHeader(201) }))
	call := func(r *http.Request) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if call(phase4ActionRequest("one", "{}")).Code != 201 {
		t.Fatal("first action failed")
	}
	r := phase4ActionRequest("one", "{}")
	r.Header.Set("Test-Principal", "principal-2")
	if w := call(r); w.Code != 503 || w.Header().Get("Retry-After") == "" {
		t.Fatal("full table did not reject another scope")
	}
	if call(phase4ActionRequest("two", "{}")).Code != 503 {
		t.Fatal("full table accepted new action")
	}
	for _, change := range []func(*http.Request){
		func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
		func(r *http.Request) { r.Method = "DELETE" },
		func(r *http.Request) { r.URL.Path = "/orders/other" },
		func(r *http.Request) { r.URL.RawQuery = "other=true" },
		func(r *http.Request) { r.Header.Set("HX-Request", "true") },
		func(r *http.Request) { r.Header.Set("HX-Target", "other-target") },
	} {
		r := phase4ActionRequest("one", "{}")
		change(r)
		if call(r).Code != 409 {
			t.Fatal("different target/method/content type reused action")
		}
	}
	time.Sleep(1100 * time.Millisecond)
	if call(phase4ActionRequest("two", "{}")).Code != 201 || count != 2 {
		t.Fatal("expired completed action not reclaimed")
	}
}

func TestPhase4IdempotencyPendingNeverExpires(t *testing.T) {
	options := phase4IdempotencyOptions()
	options.MaxEntries, options.TTL = 1, time.Millisecond
	adapter, _ := Idempotency(options)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(201)
	}))
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), phase4ActionRequest("one", "{}"))
	}()
	<-started
	time.Sleep(5 * time.Millisecond)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, phase4ActionRequest("two", "{}"))
	close(release)
	<-done
	if w.Code != 503 {
		t.Fatal("pending action evicted on TTL")
	}
}

func TestPhase4IdempotencyInputBounds(t *testing.T) {
	options := phase4IdempotencyOptions()
	options.MaxBodyBytes = 4
	adapter, _ := Idempotency(options)
	handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) }))
	for _, test := range []struct {
		change func(*http.Request)
		status int
	}{
		{func(r *http.Request) { r.Header.Del("Idempotency-Key") }, 400},
		{func(r *http.Request) { r.Header.Set("Idempotency-Key", "with space") }, 400},
		{func(r *http.Request) { r.Header.Add("Idempotency-Key", "duplicate") }, 400},
		{func(r *http.Request) { r.Header.Del("Test-Principal") }, 401},
		{func(r *http.Request) { r.Method = "GET" }, 405},
		{func(r *http.Request) { r.Header.Set("Upgrade", "websocket") }, 400},
		{func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader("12345")); r.ContentLength = -1 }, 413},
		{func(r *http.Request) { r.ContentLength = 5 }, 413},
		{func(r *http.Request) { r.Header.Set("Content-Type", "invalid/type; broken") }, 400},
	} {
		r := phase4ActionRequest("one", "{}")
		test.change(r)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Errorf("got %d want %d", w.Code, test.status)
		}
	}
}

func TestPhase4IdempotencyWaitBounds(t *testing.T) {
	options := phase4IdempotencyOptions()
	options.MaxWaiters, options.WaitTimeout = 1, 30*time.Millisecond
	adapter, err := Idempotency(options)
	if err != nil {
		t.Fatal(err)
	}
	var count atomic.Int32
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		close(started)
		<-release
		w.WriteHeader(201)
	}))
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), phase4ActionRequest("same", "{}"))
	}()
	<-started
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, phase4ActionRequest("same", "{}"))
			if w.Code != 503 || w.Header().Get("Retry-After") == "" {
				t.Errorf("bounded duplicate wait got %d", w.Code)
			}
		}()
	}
	group.Wait()
	close(release)
	<-done
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, phase4ActionRequest("same", "{}"))
	if w.Code != 201 || count.Load() != 1 {
		t.Fatal("wait timeout reran or invalidated committed mutation")
	}
}

func TestPhase4IdempotencyPrincipalIsolation(t *testing.T) {
	adapter, _ := Idempotency(phase4IdempotencyOptions())
	var count int
	handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		_, _ = io.WriteString(w, r.Header.Get("Test-Principal"))
	}))
	for _, principal := range []string{"first", "second", "first", "second"} {
		r := phase4ActionRequest("same", "{}")
		r.Header.Set("Test-Principal", principal)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Body.String() != principal {
			t.Fatal("cross-principal response replay")
		}
	}
	if count != 2 {
		t.Fatalf("principal executions %d want 2", count)
	}
}

func TestPhase4IdempotencyValidationBeforeReservation(t *testing.T) {
	options := phase4IdempotencyOptions()
	options.Validate = func(r *http.Request) error {
		body, _ := io.ReadAll(r.Body)
		if string(body) == "invalid" {
			return &httpx.Error{Status: http.StatusBadRequest}
		}
		return nil
	}
	adapter, _ := Idempotency(options)
	var count int
	handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		body, _ := io.ReadAll(r.Body)
		if string(body) != "{}" {
			t.Fatalf("validation consumed mutation body %q", body)
		}
		w.WriteHeader(201)
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, phase4ActionRequest("same", "invalid"))
	if w.Code != 400 {
		t.Fatal("precondition validation not applied")
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, phase4ActionRequest("same", "{}"))
	if w.Code != 201 || count != 1 {
		t.Fatal("validation failure reserved action ID")
	}
}
