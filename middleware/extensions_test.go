package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPhase4AdmissionAndRate(t *testing.T) {
	admit, err := AdmissionLimit(AdmissionOptions{MaxConcurrent: 1, RetryAfter: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	start, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	handler := admit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(start)
		<-release
	}))
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	<-start
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))
	if recorder.Code != 503 || recorder.Header().Get("Retry-After") != "1" {
		t.Fatalf("admission got %d %v", recorder.Code, recorder.Header())
	}
	close(release)
	<-done
	panicHandler := admit(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("test") }))
	func() {
		defer func() { _ = recover() }()
		panicHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	recorder = httptest.NewRecorder()
	admit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) })).
		ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))
	if recorder.Code != 201 {
		t.Fatal("panic leaked admission slot")
	}

	rate, err := RateLimit(RateLimitOptions{MaxEntries: 1, Requests: 3, Window: time.Second,
		Key: func(r *http.Request) (string, error) { return r.Header.Get("Test-Key"), nil }})
	if err != nil {
		t.Fatal(err)
	}
	var passed atomic.Int32
	limited := rate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { passed.Add(1) }))
	request := func(key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Test-Key", key)
		w := httptest.NewRecorder()
		limited.ServeHTTP(w, r)
		return w
	}
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() { defer group.Done(); request("one") }()
	}
	group.Wait()
	if passed.Load() != 3 {
		t.Fatalf("rate passed %d", passed.Load())
	}
	if response := request("one"); response.Code != 429 || response.Header().Get("Retry-After") == "" {
		t.Fatal("expected bounded key rejection")
	}
	if request("two").Code != 503 {
		t.Fatal("full rate table accepted new key")
	}
	time.Sleep(1100 * time.Millisecond)
	if request("two").Code != 200 {
		t.Fatal("expired rate entry not reclaimed")
	}
}

func TestPhase4CORSAndHSTS(t *testing.T) {
	cors, err := CORS(CORSOptions{Origins: []string{"https://app.example"}, Methods: []string{"POST"},
		Headers: []string{"Content-Type"}, Credentials: true, MaxAge: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		origin, method, header string
		status                 int
	}{
		{"https://app.example", "POST", "content-type", 204},
		{"https://app.example.evil", "POST", "content-type", 403},
		{"https://app.example", "DELETE", "content-type", 403},
		{"https://app.example", "POST", "authorization", 403},
		{"null", "POST", "", 403},
	} {
		r := httptest.NewRequest("OPTIONS", "/", nil)
		r.Header.Set("Origin", test.origin)
		r.Header.Set("Access-Control-Request-Method", test.method)
		r.Header.Set("Access-Control-Request-Headers", test.header)
		w := httptest.NewRecorder()
		cors(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("preflight reached handler")
		})).ServeHTTP(w, r)
		if w.Code != test.status {
			t.Errorf("CORS %+v got %d", test, w.Code)
		}
		if !headerToken(strings.Join(w.Header().Values("Vary"), ","), "Access-Control-Request-Headers") {
			t.Fatal("preflight missing vary")
		}
	}
	if _, err := CORS(CORSOptions{Origins: []string{"*"}, Methods: []string{"GET"}, Credentials: true}); err == nil {
		t.Fatal("wildcard credentials accepted")
	}
	hsts, _ := HSTS(HSTSOptions{MaxAge: time.Hour})
	for _, secure := range []bool{false, true} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-Forwarded-Proto", "https")
		if secure {
			r.TLS = &tls.ConnectionState{}
		}
		w := httptest.NewRecorder()
		hsts(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(w, r)
		if (w.Header().Get("Strict-Transport-Security") != "") != secure {
			t.Fatal("HSTS trusted spoofed scheme")
		}
	}
}

func TestPhase4ExtensionConfiguration(t *testing.T) {
	check := func(_ Extension, err error) {
		t.Helper()
		if err == nil {
			t.Error("invalid configuration accepted")
		}
	}
	check(AdmissionLimit(AdmissionOptions{}))
	check(RateLimit(RateLimitOptions{}))
	check(HSTS(HSTSOptions{}))
	check(CORS(CORSOptions{}))
	check(Idempotency(IdempotencyOptions{}))
	for _, header := range []string{"Set-Cookie", "Server-Timing", "X-Request-ID", "Cache-Control", "Content-Security-Policy", "Access-Control-Allow-Origin", "Traceparent", "Trailer"} {
		options := phase4IdempotencyOptions()
		options.ReplayHeaders = []string{header}
		check(Idempotency(options))
	}
}
