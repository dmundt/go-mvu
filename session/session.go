// Package session provides signed, expiring opaque session IDs.
//
// IDs are model keys, not models. Rotation and invalidation do not migrate or
// delete application models or CSRF state. Applications must do that explicitly.
package session

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type contextKey struct{}

var (
	ErrWeakSecret        = errors.New("session signing secret must be at least 32 bytes")
	ErrInvalidOptions    = errors.New("invalid session options")
	ErrNoSession         = errors.New("request has no session for this middleware")
	ErrInvalidSession    = errors.New("session is expired, revoked, or invalidated")
	ErrResponseCommitted = errors.New("session lifecycle change after response committed")
	ErrMigrationRequired = errors.New("session rotation requires an explicit migration callback")
)

const (
	// DefaultLifetime is the absolute server-enforced lifetime, even for browser
	// session cookies (MaxAge == 0). Requests do not extend it.
	DefaultLifetime = 24 * time.Hour
)

type Options struct {
	CookieName string
	Path       string
	Secure     bool
	SameSite   http.SameSite
	// MaxAge controls browser persistence only. Zero creates a browser-session
	// cookie; positive values must not exceed Lifetime. Negative values are invalid.
	MaxAge int
	// Lifetime defaults to DefaultLifetime and must be a positive whole number
	// of seconds. Expiration is signed and checked by the server.
	Lifetime time.Duration
	// VerificationKeys are explicitly trusted old signing keys, each at least
	// 32 bytes. They never sign new cookies. Keys are copied by New.
	VerificationKeys [][]byte
	// Clock defaults to time.Now. A custom clock must be concurrency-safe,
	// nondecreasing, and within the supported HTTP cookie date range.
	Clock func() time.Time
	// Revocations defaults to a new MemoryRevocations with capacity 10000.
	// Share this instance across middleware instances during key rollover.
	// Its guarantees are process-local and lost when the backend is replaced.
	Revocations *MemoryRevocations
}

type Middleware struct {
	secret       []byte
	verification [][]byte
	options      Options
}

type token struct {
	id      string
	expires time.Time
}

type requestState struct {
	mu         sync.Mutex
	middleware *Middleware
	token      token
	pending    *http.Cookie
	committed  bool
	invalid    bool
}

func New(secret []byte, options Options) (*Middleware, error) {
	if len(secret) < 32 {
		return nil, ErrWeakSecret
	}
	if options.CookieName == "" {
		options.CookieName = "gomvu_session"
	}
	if options.Path == "" {
		options.Path = "/"
	}
	if options.SameSite == 0 {
		options.SameSite = http.SameSiteLaxMode
	}
	if options.Lifetime == 0 {
		options.Lifetime = DefaultLifetime
	}
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.Lifetime < time.Second || options.Lifetime%time.Second != 0 ||
		options.MaxAge < 0 || int64(options.MaxAge) > int64(options.Lifetime/time.Second) ||
		options.SameSite < http.SameSiteDefaultMode || options.SameSite > http.SameSiteNoneMode ||
		(options.SameSite == http.SameSiteNoneMode && !options.Secure) ||
		!strings.HasPrefix(options.Path, "/") {
		return nil, ErrInvalidOptions
	}
	cookie := &http.Cookie{Name: options.CookieName, Value: "validation", Path: options.Path}
	if err := cookie.Valid(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidOptions, err)
	}
	if options.Revocations == nil {
		options.Revocations, _ = NewMemoryRevocations(10000)
	} else if options.Revocations.capacity <= 0 {
		return nil, fmt.Errorf("%w: uninitialized revocations", ErrInvalidOptions)
	}
	verification := make([][]byte, 0, len(options.VerificationKeys)+1)
	verification = append(verification, append([]byte(nil), secret...))
	for _, key := range options.VerificationKeys {
		if len(key) < 32 {
			return nil, ErrWeakSecret
		}
		verification = append(verification, append([]byte(nil), key...))
	}
	options.VerificationKeys = nil
	return &Middleware{secret: verification[0], verification: verification, options: options}, nil
}

// Wrap replaces missing, malformed, legacy, expired, or revoked cookies with a
// new session. It does not silently renew valid sessions or re-sign old-key
// cookies. The pending cookie is emitted on the first write, or handler return.
// Internal issuance failures are logged with slog.ErrorContext and returned as
// generic HTTP 500 responses without invoking the wrapped handler.
func (middleware *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := middleware.options.Clock()
		current, valid := middleware.readToken(r, now)
		state := &requestState{middleware: middleware, token: current}
		if !valid {
			var err error
			state.token, err = middleware.newToken(now)
			if err != nil {
				slog.ErrorContext(r.Context(), "session issuance failed", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			state.pending = middleware.cookie(state.token)
		}
		writer := &responseWriter{ResponseWriter: w, state: state}
		ctx := context.WithValue(r.Context(), contextKey{}, state)
		next.ServeHTTP(writer, r.WithContext(ctx))
		writer.flushCookie()
	})
}

// ID returns the current ID, including the new ID after successful rotation.
// It returns false after invalidation. It is not an authorization check:
// requests already admitted by Wrap may outlive expiry or concurrent revocation.
func ID(r *http.Request) (string, bool) {
	state, ok := r.Context().Value(contextKey{}).(*requestState)
	if !ok {
		return "", false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.invalid || state.token.id == "" {
		return "", false
	}
	return state.token.id, true
}

// Rotation identifies the explicit application-model/CSRF rekey operation.
type Rotation struct {
	OldID string
	NewID string
}

// Rotate revokes the old ID and issues a fresh ID and lifetime only if migrate
// succeeds. The callback must explicitly migrate/rekey application state (or
// explicitly discard it); no model or CSRF state is carried over automatically.
//
// Call before writing the response, using the writer/request supplied by Wrap.
// migrate must not write the response or reenter this request's session APIs.
// Concurrent lifecycle operations on the same old ID are serialized. A failing
// callback leaves the old cookie valid and issues no replacement; the callback
// must leave application state unchanged on error. Panics also release the
// revocation reservation, but application rollback remains the caller's job.
func (middleware *Middleware) Rotate(w http.ResponseWriter, r *http.Request, migrate func(oldID, newID string) error) (Rotation, error) {
	state, err := middleware.state(r)
	if err != nil {
		return Rotation{}, err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if err := middleware.canChange(state); err != nil {
		return Rotation{}, err
	}
	if migrate == nil {
		return Rotation{}, ErrMigrationRequired
	}
	next, err := middleware.newToken(middleware.options.Clock())
	if err != nil {
		return Rotation{}, err
	}
	rotation := Rotation{OldID: state.token.id, NewID: next.id}
	err = middleware.options.Revocations.transition(state.token, middleware.options.Clock, func() error {
		return migrate(rotation.OldID, rotation.NewID)
	})
	if err != nil {
		// A failed migration must not cause even an initial pending cookie to
		// appear in an error response.
		state.pending = nil
		return Rotation{}, err
	}
	next.expires = middleware.options.Clock().Add(middleware.options.Lifetime).Truncate(time.Second)
	state.token = next
	state.pending = nil
	http.SetCookie(w, middleware.cookie(next))
	return rotation, nil
}

// Invalidate revokes the current ID until its signed expiry and deletes its
// browser cookie. Copied cookies are rejected by middleware sharing Revocations.
// It does not delete models or CSRF state, nor cancel already admitted requests.
// Call before writing the response. On error, no deletion cookie is emitted.
func (middleware *Middleware) Invalidate(w http.ResponseWriter, r *http.Request) error {
	state, err := middleware.state(r)
	if err != nil {
		return err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if err := middleware.canChange(state); err != nil {
		return err
	}
	if err := middleware.options.Revocations.transition(state.token, middleware.options.Clock, nil); err != nil {
		state.pending = nil
		return err
	}
	state.invalid = true
	state.pending = nil
	cookie := middleware.cookie(token{})
	cookie.Value = ""
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0)
	http.SetCookie(w, cookie)
	return nil
}

func (middleware *Middleware) state(r *http.Request) (*requestState, error) {
	state, ok := r.Context().Value(contextKey{}).(*requestState)
	if !ok || state.middleware != middleware {
		return nil, ErrNoSession
	}
	return state, nil
}

func (middleware *Middleware) canChange(state *requestState) error {
	if state.committed {
		return ErrResponseCommitted
	}
	if state.invalid || !middleware.options.Clock().Before(state.token.expires) {
		return ErrInvalidSession
	}
	return nil
}

func (middleware *Middleware) readToken(r *http.Request, now time.Time) (token, bool) {
	cookie, err := r.Cookie(middleware.options.CookieName)
	if err != nil || len(cookie.Value) > 128 {
		return token{}, false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 4 || parts[0] != "v1" {
		return token{}, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || len(signature) != sha256.Size {
		return token{}, false
	}
	payload := strings.Join(parts[:3], ".")
	valid := false
	for _, key := range middleware.verification {
		valid = hmac.Equal(signature, sign(key, payload)) || valid
	}
	if !valid {
		return token{}, false
	}
	id, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(id) != 32 || base64.RawURLEncoding.EncodeToString(id) != parts[1] {
		return token{}, false
	}
	seconds, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || seconds <= 0 || strconv.FormatInt(seconds, 10) != parts[2] {
		return token{}, false
	}
	current := token{id: string(id), expires: time.Unix(seconds, 0)}
	revoked := middleware.options.Revocations.revoked(current.id, middleware.options.Clock)
	if !now.Before(current.expires) || revoked {
		return token{}, false
	}
	// Waiting for an in-flight migration must not admit an expired session.
	return current, middleware.options.Clock().Before(current.expires)
}

func (middleware *Middleware) newToken(now time.Time) (token, error) {
	expiry := now.Add(middleware.options.Lifetime).Truncate(time.Second)
	if now.Unix() < 0 || !now.Before(expiry) || expiry.Year() > 9999 {
		return token{}, errors.New("session clock outside supported cookie expiry range")
	}
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return token{}, err
	}
	return token{id: string(value), expires: expiry}, nil
}

func (middleware *Middleware) cookie(current token) *http.Cookie {
	payload := "v1." + base64.RawURLEncoding.EncodeToString([]byte(current.id)) +
		"." + strconv.FormatInt(current.expires.Unix(), 10)
	cookie := &http.Cookie{
		Name:  middleware.options.CookieName,
		Value: payload + "." + base64.RawURLEncoding.EncodeToString(sign(middleware.secret, payload)),
		Path:  middleware.options.Path, MaxAge: middleware.options.MaxAge,
		HttpOnly: true, Secure: middleware.options.Secure, SameSite: middleware.options.SameSite,
	}
	if cookie.MaxAge > 0 {
		cookie.Expires = current.expires
	}
	return cookie
}

func sign(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

type responseWriter struct {
	http.ResponseWriter
	state *requestState
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *responseWriter) flushCookie() {
	w.state.mu.Lock()
	defer w.state.mu.Unlock()
	if w.state.pending != nil {
		http.SetCookie(w.ResponseWriter, w.state.pending)
		w.state.pending = nil
	}
}

func (w *responseWriter) WriteHeader(code int) {
	if code >= 100 && code < 200 && code != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	w.flushCookie()
	w.state.mu.Lock()
	w.state.committed = true
	w.state.mu.Unlock()
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(value []byte) (int, error) {
	w.flushCookie()
	w.state.mu.Lock()
	w.state.committed = true
	w.state.mu.Unlock()
	return w.ResponseWriter.Write(value)
}

func (w *responseWriter) FlushError() error {
	w.state.mu.Lock()
	committed := w.state.committed
	w.state.mu.Unlock()
	if !committed {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *responseWriter) Flush() { _ = w.FlushError() }
