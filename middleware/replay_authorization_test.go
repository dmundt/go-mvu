package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/session"
)

type replayPrincipal struct {
	account string
	tenant  string
	session string
}
type replayPrincipalKey struct{}

func TestReplayRechecksCurrentPermissionsAndTenant(t *testing.T) {
	var allowed atomic.Bool
	allowed.Store(true)
	var calls atomic.Int32
	options := phase4IdempotencyOptions()
	options.Scope = func(r *http.Request) (string, error) {
		p, ok := r.Context().Value(replayPrincipalKey{}).(replayPrincipal)
		if !ok || p.account == "" {
			return "", errors.New("not authenticated")
		}
		return p.tenant + "/" + p.account + "/" + p.session, nil
	}
	options.Validate = func(*http.Request) error {
		if !allowed.Load() {
			return &httpx.Error{Status: http.StatusForbidden}
		}
		return nil
	}
	wrap, err := Idempotency(options)
	if err != nil {
		t.Fatal(err)
	}
	handler := wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, "private result")
	}))
	do := func(p replayPrincipal) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		request := phase4ActionRequest("same-action", "{}")
		request = request.WithContext(context.WithValue(request.Context(), replayPrincipalKey{}, p))
		handler.ServeHTTP(w, request)
		return w
	}
	owner := replayPrincipal{"user-1", "tenant-1", "session-1"}
	if w := do(owner); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := do(owner); w.Code != 200 || calls.Load() != 1 {
		t.Fatal("authorized duplicate not replayed")
	}
	allowed.Store(false)
	if w := do(owner); w.Code != 403 || strings.Contains(w.Body.String(), "private result") {
		t.Fatal("revoked permission disclosed cached result")
	}
	if w := do(replayPrincipal{}); w.Code != 401 {
		t.Fatal("logout not enforced")
	}
	allowed.Store(true)
	for _, p := range []replayPrincipal{
		{"user-2", "tenant-1", "session-1"},
		{"user-1", "tenant-2", "session-1"},
		{"user-1", "tenant-1", "session-2"},
	} {
		before := calls.Load()
		if w := do(p); w.Code != 200 || calls.Load() != before+1 {
			t.Fatal("scope change reused another principal's result")
		}
	}
}

func TestWaitingReplayRevalidatesAfterAccessRevocation(t *testing.T) {
	var allowed atomic.Bool
	allowed.Store(true)
	var validationCalls atomic.Int32
	entered, release, waiterChecked := make(chan struct{}), make(chan struct{}), make(chan struct{})
	options := phase4IdempotencyOptions()
	options.Validate = func(*http.Request) error {
		if validationCalls.Add(1) == 2 {
			close(waiterChecked)
		}
		if !allowed.Load() {
			return &httpx.Error{Status: 403}
		}
		return nil
	}
	wrap, err := Idempotency(options)
	if err != nil {
		t.Fatal(err)
	}
	handler := wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, "private result")
	}))
	done, duplicateDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), phase4ActionRequest("same", "{}"))
	}()
	<-entered
	duplicate := httptest.NewRecorder()
	go func() {
		defer close(duplicateDone)
		handler.ServeHTTP(duplicate, phase4ActionRequest("same", "{}"))
	}()
	<-waiterChecked
	allowed.Store(false)
	close(release)
	<-done
	<-duplicateDone
	if duplicate.Code != 403 || strings.Contains(duplicate.Body.String(), "private result") {
		t.Fatalf("waiting replay disclosed result: %d %s", duplicate.Code, duplicate.Body.String())
	}
}

func TestReplayRejectsExpiredAndInvalidatedSessions(t *testing.T) {
	var clockMu sync.Mutex
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return now }
	sessions, err := session.New([]byte(strings.Repeat("k", 32)), session.Options{
		Lifetime: time.Minute, Clock: clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	var owner string
	var calls int
	options := phase4IdempotencyOptions()
	options.Scope = func(r *http.Request) (string, error) {
		id, ok := session.ID(r)
		if !ok || id != owner {
			return "", errors.New("session not authenticated")
		}
		return id, nil
	}
	wrap, err := Idempotency(options)
	if err != nil {
		t.Fatal(err)
	}
	handler := sessions.Wrap(wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = io.WriteString(w, "private")
	})))
	issue := httptest.NewRecorder()
	sessions.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner, _ = session.ID(r)
	})).ServeHTTP(issue, httptest.NewRequest("GET", "/", nil))
	cookie := issue.Result().Cookies()[0]
	action := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := phase4ActionRequest("same", "{}")
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
		return w
	}
	if action().Code != 200 {
		t.Fatal("initial action failed")
	}
	logout := httptest.NewRequest("POST", "/logout", nil)
	logout.AddCookie(cookie)
	sessions.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.Invalidate(w, r); err != nil {
			t.Fatal(err)
		}
	})).ServeHTTP(httptest.NewRecorder(), logout)
	if w := action(); w.Code != 401 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("invalidated cookie replayed")
	}
	clockMu.Lock()
	now = now.Add(2 * time.Minute)
	clockMu.Unlock()
	if w := action(); w.Code != 401 || calls != 1 {
		t.Fatal("expired cookie replayed or reran action")
	}
}
