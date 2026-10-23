package session

import (
	"bytes"
	"crypto/hmac"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var testSecret = []byte(strings.Repeat("s", 32))

type testClock struct{ seconds atomic.Int64 }

func newTestClock() *testClock {
	clock := &testClock{}
	clock.seconds.Store(1800000000)
	return clock
}

func (clock *testClock) now() time.Time    { return time.Unix(clock.seconds.Load(), 0) }
func (clock *testClock) add(seconds int64) { clock.seconds.Add(seconds) }

func mustMiddleware(t *testing.T, options Options) *Middleware {
	t.Helper()
	middleware, err := New(testSecret, options)
	if err != nil {
		t.Fatal(err)
	}
	return middleware
}

func serveSession(middleware *Middleware, cookie *http.Cookie, handler http.HandlerFunc) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	middleware.Wrap(handler).ServeHTTP(response, request)
	return response
}

func issue(t *testing.T, middleware *Middleware) (*http.Cookie, string) {
	t.Helper()
	var id string
	response := serveSession(middleware, nil, func(w http.ResponseWriter, r *http.Request) {
		id, _ = ID(r)
	})
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("issuance cookies = %v, status = %d", cookies, response.Code)
	}
	return cookies[0], id
}

func requestID(t *testing.T, middleware *Middleware, cookie *http.Cookie) (string, *httptest.ResponseRecorder) {
	t.Helper()
	var id string
	response := serveSession(middleware, cookie, func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		id, ok = ID(r)
		if !ok {
			t.Error("missing ID")
		}
	})
	return id, response
}

func TestServerExpiryAndLegacyRejection(t *testing.T) {
	clock := newTestClock()
	middleware := mustMiddleware(t, Options{Clock: clock.now})
	cookie, id := issue(t, middleware)
	if cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
		t.Fatalf("default browser-session cookie = %+v", cookie)
	}
	clock.add(int64(DefaultLifetime/time.Second) - 1)
	if got, response := requestID(t, middleware, cookie); got != id || len(response.Result().Cookies()) != 0 {
		t.Fatal("valid cookie renewed or replaced before absolute expiry")
	}
	clock.add(1)
	if got, response := requestID(t, middleware, cookie); got == id || len(response.Result().Cookies()) != 1 {
		t.Fatal("copied cookie accepted at signed expiry")
	}
	encoded := base64.RawURLEncoding.EncodeToString([]byte(id))
	legacy := &http.Cookie{Name: cookie.Name, Value: encoded + "." +
		base64.RawURLEncoding.EncodeToString(sign(testSecret, encoded))}
	if got, _ := requestID(t, middleware, legacy); got == id {
		t.Fatal("legacy cookie accepted")
	}
}

func TestExpiryIsSignedAndCookieOptionsPreserved(t *testing.T) {
	clock := newTestClock()
	middleware := mustMiddleware(t, Options{
		Clock: clock.now, Lifetime: time.Hour, MaxAge: 600,
		CookieName: "custom", Path: "/app", Secure: true, SameSite: http.SameSiteStrictMode,
	})
	cookie, id := issue(t, middleware)
	if cookie.Name != "custom" || cookie.Path != "/app" || !cookie.Secure || !cookie.HttpOnly ||
		cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != 600 || !cookie.Expires.Equal(clock.now().Add(time.Hour)) {
		t.Fatalf("cookie = %+v", cookie)
	}
	parts := strings.Split(cookie.Value, ".")
	parts[2] = strconv.FormatInt(clock.now().Add(48*time.Hour).Unix(), 10)
	cookie.Value = strings.Join(parts, ".")
	if got, _ := requestID(t, middleware, cookie); got == id {
		t.Fatal("tampered signed expiry accepted")
	}
}

func TestKeyRolloverRequiresExplicitKeysAndSharesRevocations(t *testing.T) {
	clock := newTestClock()
	revocations, _ := NewMemoryRevocations(10)
	old := mustMiddleware(t, Options{Clock: clock.now, Revocations: revocations})
	cookie, id := issue(t, old)
	newKey := []byte(strings.Repeat("n", 32))
	oldKey := append([]byte(nil), testSecret...)
	current, err := New(newKey, Options{
		Clock: clock.now, VerificationKeys: [][]byte{oldKey}, Revocations: revocations,
	})
	if err != nil {
		t.Fatal(err)
	}
	newKey[0], oldKey[0] = 'x', 'x'
	if got, response := requestID(t, current, cookie); got != id || len(response.Result().Cookies()) != 0 {
		t.Fatal("explicit old verification key not accepted without renewal")
	}
	currentCookie, _ := issue(t, current)
	parts := strings.Split(currentCookie.Value, ".")
	signature, _ := base64.RawURLEncoding.DecodeString(parts[3])
	payload := strings.Join(parts[:3], ".")
	if !hmac.Equal(signature, sign([]byte(strings.Repeat("n", 32)), payload)) {
		t.Fatal("current key not used for signing or caller changed copied key")
	}
	currentOnly, _ := New([]byte(strings.Repeat("n", 32)), Options{Clock: clock.now})
	if got, _ := requestID(t, currentOnly, cookie); got == id {
		t.Fatal("unconfigured old verification key accepted")
	}
	response := serveSession(current, cookie, func(w http.ResponseWriter, r *http.Request) {
		if err := current.Invalidate(w, r); err != nil {
			t.Fatal(err)
		}
	})
	if response.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("invalidation did not delete cookie")
	}
	if got, _ := requestID(t, old, cookie); got == id {
		t.Fatal("shared backend did not revoke old-key cookie")
	}
}

func TestRotateExplicitlyMigratesAndRevokesOldID(t *testing.T) {
	clock := newTestClock()
	middleware := mustMiddleware(t, Options{Clock: clock.now, Lifetime: time.Hour})
	cookie, oldID := issue(t, middleware)
	models := map[string]string{oldID: "model with old CSRF binding"}
	clock.add(10)
	var rotation Rotation
	response := serveSession(middleware, cookie, func(w http.ResponseWriter, r *http.Request) {
		var err error
		rotation, err = middleware.Rotate(w, r, func(oldID, newID string) error {
			if oldID == newID || len(newID) != 32 {
				t.Fatal("rotation did not generate a fresh opaque ID")
			}
			models[newID] = models[oldID]
			delete(models, oldID)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, ok := ID(r); !ok || got != rotation.NewID {
			t.Fatal("context ID not updated after migration")
		}
	})
	if rotation.OldID != oldID || models[rotation.NewID] == "" || models[oldID] != "" {
		t.Fatal("explicit model migration failed")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("rotation cookies = %v", cookies)
	}
	if got, _ := requestID(t, middleware, cookies[0]); got != rotation.NewID {
		t.Fatal("rotated cookie rejected")
	}
	if got, _ := requestID(t, middleware, cookie); got == oldID {
		t.Fatal("copied old cookie accepted after rotation")
	}
	clock.add(3590)
	if got, _ := requestID(t, middleware, cookies[0]); got != rotation.NewID {
		t.Fatal("rotation did not reset server lifetime")
	}
	clock.add(10)
	if got, _ := requestID(t, middleware, cookies[0]); got == rotation.NewID {
		t.Fatal("rotated cookie accepted at new expiry")
	}
}

func TestMigrationErrorDoesNotIssueOrRevoke(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(strconv.FormatBool(existing), func(t *testing.T) {
			middleware := mustMiddleware(t, Options{})
			var cookie *http.Cookie
			var oldID string
			if existing {
				cookie, oldID = issue(t, middleware)
			}
			failure := errors.New("model migration failed")
			response := serveSession(middleware, cookie, func(w http.ResponseWriter, r *http.Request) {
				before, _ := ID(r)
				rotation, err := middleware.Rotate(w, r, func(oldID, newID string) error { return failure })
				if !errors.Is(err, failure) || rotation != (Rotation{}) {
					t.Fatalf("rotation = %+v, error = %v", rotation, err)
				}
				if after, ok := ID(r); !ok || after != before {
					t.Fatal("failed migration changed request ID")
				}
				http.Error(w, "migration failed", http.StatusInternalServerError)
			})
			if response.Code != http.StatusInternalServerError || len(response.Result().Cookies()) != 0 {
				t.Fatalf("failed migration response = %d, %v", response.Code, response.Result().Cookies())
			}
			if existing {
				if got, _ := requestID(t, middleware, cookie); got != oldID {
					t.Fatal("failed migration revoked old cookie")
				}
			}
		})
	}
}

func TestInvalidateRejectsCopiedCookie(t *testing.T) {
	middleware := mustMiddleware(t, Options{})
	cookie, oldID := issue(t, middleware)
	response := serveSession(middleware, cookie, func(w http.ResponseWriter, r *http.Request) {
		if err := middleware.Invalidate(w, r); err != nil {
			t.Fatal(err)
		}
		if id, ok := ID(r); ok || id != "" {
			t.Fatal("invalidated request still has a usable ID")
		}
		if err := middleware.Invalidate(w, r); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("second invalidation = %v", err)
		}
	})
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 || cookies[0].Value != "" || !cookies[0].HttpOnly ||
		cookies[0].Path != cookie.Path || !cookies[0].Expires.Before(time.Now()) {
		t.Fatalf("deletion cookie = %v", cookies)
	}
	if got, _ := requestID(t, middleware, cookie); got == oldID {
		t.Fatal("copied cookie accepted after invalidation")
	}
}

func TestBoundedRevocationsFailClosedAndCleanup(t *testing.T) {
	clock := newTestClock()
	backend, _ := NewMemoryRevocations(1)
	middleware := mustMiddleware(t, Options{Clock: clock.now, Lifetime: 10 * time.Second, Revocations: backend})
	first, _ := issue(t, middleware)
	serveSession(middleware, first, func(w http.ResponseWriter, r *http.Request) {
		if err := middleware.Invalidate(w, r); err != nil {
			t.Fatal(err)
		}
	})
	clock.add(1)
	second, secondID := issue(t, middleware)
	response := serveSession(middleware, second, func(w http.ResponseWriter, r *http.Request) {
		called := false
		_, err := middleware.Rotate(w, r, func(oldID, newID string) error { called = true; return nil })
		if !errors.Is(err, ErrRevocationCapacity) || called {
			t.Fatalf("capacity error = %v, migration called = %v", err, called)
		}
		if err := middleware.Invalidate(w, r); !errors.Is(err, ErrRevocationCapacity) {
			t.Fatalf("invalidation capacity error = %v", err)
		}
		http.Error(w, "revocation unavailable", http.StatusServiceUnavailable)
	})
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("failed lifecycle operation changed cookie")
	}
	if got, _ := requestID(t, middleware, second); got != secondID {
		t.Fatal("capacity failure changed old session")
	}
	clock.add(9)
	serveSession(middleware, second, func(w http.ResponseWriter, r *http.Request) {
		if err := middleware.Invalidate(w, r); err != nil {
			t.Fatalf("expired revocation capacity not reclaimed: %v", err)
		}
	})
	clock.add(1)
	_, _ = requestID(t, middleware, second)
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if len(backend.entries) != 0 {
		t.Fatal("expired revocation retained")
	}
}

func TestLifecycleValidation(t *testing.T) {
	middleware := mustMiddleware(t, Options{})
	other := mustMiddleware(t, Options{})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := middleware.Invalidate(httptest.NewRecorder(), request); !errors.Is(err, ErrNoSession) {
		t.Fatalf("missing context error = %v", err)
	}
	serveSession(middleware, nil, func(w http.ResponseWriter, r *http.Request) {
		if _, err := other.Rotate(w, r, func(string, string) error { return nil }); !errors.Is(err, ErrNoSession) {
			t.Fatalf("wrong middleware error = %v", err)
		}
		if _, err := middleware.Rotate(w, r, nil); !errors.Is(err, ErrMigrationRequired) {
			t.Fatalf("nil migration error = %v", err)
		}
		_, _ = w.Write([]byte("committed"))
		if _, err := middleware.Rotate(w, r, func(string, string) error { return nil }); !errors.Is(err, ErrResponseCommitted) {
			t.Fatalf("committed rotation error = %v", err)
		}
		if err := middleware.Invalidate(w, r); !errors.Is(err, ErrResponseCommitted) {
			t.Fatalf("committed invalidation error = %v", err)
		}
	})
}

func TestConfigurationValidation(t *testing.T) {
	for name, options := range map[string]Options{
		"weak verification key": {VerificationKeys: [][]byte{[]byte("weak")}},
		"negative lifetime":     {Lifetime: -time.Second},
		"subsecond lifetime":    {Lifetime: time.Millisecond},
		"fractional lifetime":   {Lifetime: time.Second + time.Millisecond},
		"negative max age":      {MaxAge: -1},
		"long max age":          {MaxAge: int(DefaultLifetime/time.Second) + 1},
		"cookie name":           {CookieName: "bad name"},
		"cookie path":           {Path: "/bad;path"},
		"relative path":         {Path: "relative"},
		"same site":             {SameSite: http.SameSite(99)},
		"insecure none":         {SameSite: http.SameSiteNoneMode},
		"zero backend":          {Revocations: &MemoryRevocations{}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(testSecret, options); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	if _, err := New([]byte("weak"), Options{}); !errors.Is(err, ErrWeakSecret) {
		t.Fatalf("weak secret error = %v", err)
	}
	if _, err := NewMemoryRevocations(0); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("invalid capacity error = %v", err)
	}
}

func TestConcurrentRotationOnlyMigratesOnce(t *testing.T) {
	middleware := mustMiddleware(t, Options{})
	cookie, _ := issue(t, middleware)
	const count = 16
	var admitted sync.WaitGroup
	admitted.Add(count)
	start := make(chan struct{})
	var done sync.WaitGroup
	var migrations, successes atomic.Int32
	for range count {
		done.Add(1)
		go func() {
			defer done.Done()
			serveSession(middleware, cookie, func(w http.ResponseWriter, r *http.Request) {
				admitted.Done()
				<-start
				_, err := middleware.Rotate(w, r, func(oldID, newID string) error {
					migrations.Add(1)
					return nil
				})
				if err == nil {
					successes.Add(1)
				} else if !errors.Is(err, ErrInvalidSession) {
					t.Errorf("concurrent rotation error = %v", err)
				}
			})
		}()
	}
	admitted.Wait()
	close(start)
	done.Wait()
	if migrations.Load() != 1 || successes.Load() != 1 {
		t.Fatalf("migrations = %d, successful rotations = %d", migrations.Load(), successes.Load())
	}
}

func TestConcurrentInvalidateAndRequests(t *testing.T) {
	middleware := mustMiddleware(t, Options{})
	cookie, oldID := issue(t, middleware)
	var admitted sync.WaitGroup
	admitted.Add(16)
	start := make(chan struct{})
	var done sync.WaitGroup
	var successes atomic.Int32
	for range 16 {
		done.Add(1)
		go func() {
			defer done.Done()
			serveSession(middleware, cookie, func(w http.ResponseWriter, r *http.Request) {
				admitted.Done()
				<-start
				err := middleware.Invalidate(w, r)
				if err == nil {
					successes.Add(1)
				} else if !errors.Is(err, ErrInvalidSession) {
					t.Errorf("concurrent invalidation = %v", err)
				}
			})
		}()
	}
	admitted.Wait()
	close(start)
	done.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful concurrent invalidations = %d", successes.Load())
	}
	if got, _ := requestID(t, middleware, cookie); got == oldID {
		t.Fatal("concurrent invalidations failed to revoke copied cookie")
	}
}

func TestReadersWaitForFailedMigrationRollback(t *testing.T) {
	middleware := mustMiddleware(t, Options{})
	cookie, oldID := issue(t, middleware)
	migrating, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		serveSession(middleware, cookie, func(w http.ResponseWriter, r *http.Request) {
			_, err := middleware.Rotate(w, r, func(string, string) error {
				close(migrating)
				<-release
				return errors.New("rollback")
			})
			if err == nil {
				t.Error("failed migration succeeded")
			}
		})
	}()
	<-migrating
	const count = 16
	var readers sync.WaitGroup
	readers.Add(count)
	for range count {
		go func() {
			defer readers.Done()
			if got, _ := requestID(t, middleware, cookie); got != oldID {
				t.Error("reader rejected cookie during failed migration")
			}
		}()
	}
	close(release)
	<-finished
	readers.Wait()
}

func TestSlowMigrationStartsFreshLifetimeAtCommit(t *testing.T) {
	clock := newTestClock()
	middleware := mustMiddleware(t, Options{Clock: clock.now, Lifetime: time.Second})
	cookie, _ := issue(t, middleware)
	var newID string
	response := serveSession(middleware, cookie, func(w http.ResponseWriter, r *http.Request) {
		rotation, err := middleware.Rotate(w, r, func(string, string) error {
			clock.add(10)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		newID = rotation.NewID
	})
	if got, _ := requestID(t, middleware, response.Result().Cookies()[0]); got != newID {
		t.Fatal("slow migration issued an already expired cookie")
	}
}

func TestExpiryWhileRequestIsAdmitted(t *testing.T) {
	clock := newTestClock()
	middleware := mustMiddleware(t, Options{Clock: clock.now, Lifetime: time.Second})
	cookie, _ := issue(t, middleware)
	response := serveSession(middleware, cookie, func(w http.ResponseWriter, r *http.Request) {
		clock.add(1)
		called := false
		_, err := middleware.Rotate(w, r, func(string, string) error { called = true; return nil })
		if !errors.Is(err, ErrInvalidSession) || called {
			t.Fatalf("expired rotation = %v, migration called = %v", err, called)
		}
		if err := middleware.Invalidate(w, r); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("expired invalidation = %v", err)
		}
	})
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("expired lifecycle operation emitted a cookie")
	}
}

func TestResponseWriterPreservesSniffingAndFlush(t *testing.T) {
	middleware := mustMiddleware(t, Options{})
	response := serveSession(middleware, nil, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	if got := response.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("implicit Content-Type = %q", got)
	}
	response = serveSession(middleware, nil, func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatal(err)
		}
		if err := middleware.Invalidate(w, r); !errors.Is(err, ErrResponseCommitted) {
			t.Fatalf("invalidation after flush = %v", err)
		}
	})
	if !response.Flushed || len(response.Result().Cookies()) != 1 {
		t.Fatal("flush did not issue pending session cookie")
	}
}

func TestMalformedCookiesStartNewSessions(t *testing.T) {
	middleware := mustMiddleware(t, Options{})
	cookie, id := issue(t, middleware)
	parts := strings.Split(cookie.Value, ".")
	for _, value := range []string{
		"", "v1.bad", cookie.Value + ".extra", strings.Repeat("x", 129),
		"v2." + strings.Join(parts[1:], "."),
		strings.Join(parts[:3], ".") + ".invalid",
		"v1.bad." + parts[2],
		"v1." + parts[1] + ".0",
		"v1." + parts[1] + ".-1",
		"v1." + parts[1] + ".99999999999999999999999999",
		"v1." + parts[1] + ".0" + parts[2],
	} {
		t.Run(value, func(t *testing.T) {
			if len(strings.Split(value, ".")) == 3 {
				value += "." + base64.RawURLEncoding.EncodeToString(sign(testSecret, value))
			}
			if got, _ := requestID(t, middleware, &http.Cookie{Name: cookie.Name, Value: value}); got == id {
				t.Fatal("malformed cookie accepted")
			}
		})
	}
}

func TestInvalidClockFailsWithoutIssuing(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	middleware := mustMiddleware(t, Options{Clock: func() time.Time { return time.Unix(-1, 0) }})
	called := false
	response := serveSession(middleware, nil, func(w http.ResponseWriter, r *http.Request) { called = true })
	if called || response.Code != http.StatusInternalServerError || len(response.Result().Cookies()) != 0 {
		t.Fatalf("invalid clock response = %d, cookies = %v, called = %v",
			response.Code, response.Result().Cookies(), called)
	}
	if !strings.Contains(logs.String(), "level=ERROR") ||
		!strings.Contains(logs.String(), "session issuance failed") ||
		!strings.Contains(logs.String(), "session clock outside supported cookie expiry range") {
		t.Fatalf("internal issuance failure was not logged explicitly: %s", logs.String())
	}
	if strings.Contains(response.Body.String(), "clock") {
		t.Fatal("internal issuance details leaked to client")
	}
}

func TestMigrationPanicReleasesReservation(t *testing.T) {
	middleware := mustMiddleware(t, Options{})
	cookie, oldID := issue(t, middleware)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("migration panic swallowed")
			}
		}()
		serveSession(middleware, cookie, func(w http.ResponseWriter, r *http.Request) {
			_, _ = middleware.Rotate(w, r, func(string, string) error { panic("migration") })
		})
	}()
	if got, _ := requestID(t, middleware, cookie); got != oldID {
		t.Fatal("migration panic left old ID revoked or transition pending")
	}
}
