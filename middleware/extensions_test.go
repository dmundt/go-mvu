package middleware

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

func TestPhase4ProxyTrust(t *testing.T) {
	extension, err := TrustedProxies([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		peer, forwarded, proto, address, scheme string
	}{
		{"192.0.2.4:123", "198.51.100.1", "https", "192.0.2.4", "http"},
		{"10.0.0.1:123", "198.51.100.1, 10.1.1.1", "https", "198.51.100.1", "https"},
		{"10.0.0.1:123", "198.51.100.1, 192.0.2.8", "https", "192.0.2.8", "https"},
		{"10.0.0.1:123", "bad, 10.1.1.1", "https, http", "10.0.0.1", "http"},
		{"10.0.0.1:123", "198.51.100.1", "javascript", "198.51.100.1", "http"},
	}
	for _, test := range tests {
		request := httptest.NewRequest("GET", "/", nil)
		request.RemoteAddr = test.peer
		request.Header.Set("X-Forwarded-For", test.forwarded)
		request.Header.Set("X-Forwarded-Proto", test.proto)
		extension(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := ClientFromContext(r.Context())
			if !ok || info.Address.String() != test.address || info.Scheme != test.scheme {
				t.Errorf("peer %s: got %+v", test.peer, info)
			}
		})).ServeHTTP(httptest.NewRecorder(), request)
	}
	if _, err := TrustedProxies(nil); err == nil {
		t.Fatal("missing allowlist accepted")
	}
	if _, err := TrustedProxies([]netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")}); err == nil {
		t.Fatal("universal trust accepted")
	}
}

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
	admit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) })).ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))
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

func TestPhase4Telemetry(t *testing.T) {
	valid := "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"
	for _, incoming := range []string{valid, strings.ToUpper(valid), "00-" + strings.Repeat("0", 32) + "-0123456789abcdef-01", valid + "-extra"} {
		calls := 0
		adapter, err := Telemetry(TelemetryOptions{Route: "users.update", Complete: func(ctx context.Context, result Completion) {
			calls++
			if result.Route != "users.update" || result.RequestID != "current" || result.Status != 201 {
				t.Errorf("completion %+v", result)
			}
		}})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/unbounded-user-value", nil)
		r.Header.Set("Traceparent", incoming)
		r = r.WithContext(httpx.WithRequestID(r.Context(), "current"))
		adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := TraceFromContext(r.Context())
			if ok != (incoming == valid) {
				t.Errorf("trace validation %q: %v", incoming, ok)
			}
			w.WriteHeader(201)
		})).ServeHTTP(httptest.NewRecorder(), r)
		if calls != 1 {
			t.Fatalf("completion called %d times", calls)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	adapter, _ := Telemetry(TelemetryOptions{Route: "panic", Complete: func(ctx context.Context, result Completion) {
		calls++
		if result.Panic != "boom" || !errors.Is(result.Err, context.Canceled) || result.Status != 500 {
			t.Errorf("panic completion %+v", result)
		}
	}})
	func() {
		defer func() {
			if recover() != "boom" {
				t.Error("panic swallowed")
			}
		}()
		adapter(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })).ServeHTTP(
			httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	}()
	if calls != 1 {
		t.Fatal("panic completion not exactly once")
	}
}

func TestPhase4GzipNegotiation(t *testing.T) {
	adapter, err := Gzip(GzipOptions{Eligible: func(*http.Request) bool { return true }, Level: gzip.DefaultCompression})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		accept, method, contentType, cache string
		status                             int
		compressed                         bool
	}{
		{"gzip", "GET", "text/plain", "", 200, true},
		{"gzip;q=0, *;q=1", "GET", "text/plain", "", 200, false},
		{"br, *;q=0.5", "GET", "text/plain", "", 200, true},
		{"gzip;q=NaN", "GET", "text/plain", "", 200, false},
		{"gzip;q=bad", "GET", "text/plain", "", 200, false},
		{"gzip", "HEAD", "text/plain", "", 200, false},
		{"gzip", "GET", "text/event-stream", "", 200, false},
		{"gzip", "GET", "image/png", "", 200, false},
		{"gzip", "GET", "text/plain", "private, no-transform", 200, false},
		{"gzip", "GET", "text/plain", "", 204, false},
		{"gzip", "GET", "text/plain", "", 304, false},
	} {
		r := httptest.NewRequest(test.method, "/", nil)
		r.Header.Set("Accept-Encoding", test.accept)
		w := httptest.NewRecorder()
		adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", test.contentType)
			w.Header().Set("Cache-Control", test.cache)
			w.Header().Set("Content-Length", "5")
			w.WriteHeader(test.status)
			if test.status == 200 && test.method != "HEAD" {
				_, _ = io.WriteString(w, "hello")
			}
		})).ServeHTTP(w, r)
		if got := w.Header().Get("Content-Encoding") == "gzip"; got != test.compressed {
			t.Errorf("%+v: compression %v", test, got)
		}
		if test.compressed {
			if w.Header().Get("Content-Length") != "" {
				t.Error("compressed length retained")
			}
			reader, err := gzip.NewReader(w.Body)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(reader)
			if err != nil || string(body) != "hello" {
				t.Fatalf("gzip body %q %v", body, err)
			}
			_ = reader.Close()
		}
	}
}

func TestPhase4GzipStreamingFlush(t *testing.T) {
	adapter, _ := Gzip(GzipOptions{Eligible: func(*http.Request) bool { return true }, Level: gzip.BestSpeed})
	release := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	server := httptest.NewServer(adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Trailer", "Test-End")
		_, _ = io.WriteString(w, "first")
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, "last")
		w.Header().Set("Test-End", "done")
	})))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	vary := strings.Join(response.Header.Values("Vary"), ",")
	if !headerToken(vary, "Accept-Encoding") || !headerToken(vary, "Origin") {
		t.Fatalf("gzip Vary did not merge: %q", vary)
	}
	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	first := make([]byte, 5)
	if _, err := io.ReadFull(reader, first); err != nil || string(first) != "first" {
		t.Fatalf("flush did not stream: %q %v", first, err)
	}
	once.Do(func() { close(release) })
	last, err := io.ReadAll(reader)
	if err != nil || string(last) != "last" || response.Trailer.Get("Test-End") != "done" {
		t.Fatalf("stream/trailer: %q %v %v", last, response.Trailer, err)
	}
}

func TestPhase4LocaleCORSAndHSTS(t *testing.T) {
	locale, err := ResolveLocale(LocaleOptions{Supported: []Locale{"en", "de"}, Default: "en",
		CookieName: "language", Precedence: []LocaleSource{LocaleCookie, LocaleAcceptLanguage}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ cookie, accept, want string }{
		{"de", "en;q=1", "de"}, {"bad", "de-DE;q=0.9,en;q=0.1", "de"},
		{"", "de;q=0,en;q=1", "en"}, {"", "de;q=NaN", "en"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Accept-Language", test.accept)
		if test.cookie != "" {
			r.AddCookie(&http.Cookie{Name: "language", Value: test.cookie})
		}
		w := httptest.NewRecorder()
		locale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, ok := LocaleFromContext(r.Context())
			if !ok || string(got) != test.want {
				t.Errorf("locale got %q want %q", got, test.want)
			}
		})).ServeHTTP(w, r)
		if !headerToken(strings.Join(w.Header().Values("Vary"), ","), "Cookie") ||
			!headerToken(strings.Join(w.Header().Values("Vary"), ","), "Accept-Language") {
			t.Fatal("locale vary missing")
		}
	}
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
		cors(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("preflight reached handler") })).ServeHTTP(w, r)
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
	check(Telemetry(TelemetryOptions{}))
	check(Gzip(GzipOptions{}))
	check(ResolveLocale(LocaleOptions{}))
	check(ResolveLocale(LocaleOptions{Supported: []Locale{"en"}, Default: "de", Precedence: []LocaleSource{LocaleCookie}}))
	check(HSTS(HSTSOptions{}))
	check(CORS(CORSOptions{}))
	check(Idempotency(IdempotencyOptions{}))
	for _, header := range []string{"Set-Cookie", "Server-Timing", "X-Request-ID", "Cache-Control", "Content-Security-Policy", "Access-Control-Allow-Origin", "Traceparent", "Trailer"} {
		options := phase4IdempotencyOptions()
		options.ReplayHeaders = []string{header}
		check(Idempotency(options))
	}
}

type phase4FailWriter struct{ header http.Header }

func (w *phase4FailWriter) Header() http.Header { return w.header }
func (w *phase4FailWriter) WriteHeader(int)     {}
func (w *phase4FailWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestPhase4GzipCloseFailure(t *testing.T) {
	var closeError error
	adapter, _ := Gzip(GzipOptions{Eligible: func(*http.Request) bool { return true }, Level: gzip.DefaultCompression,
		OnError: func(_ *http.Request, err error) { closeError = err }})
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	adapter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "data")
	})).ServeHTTP(&phase4FailWriter{header: make(http.Header)}, r)
	if !errors.Is(closeError, io.ErrClosedPipe) {
		t.Fatalf("close error not observed: %v", closeError)
	}
}
