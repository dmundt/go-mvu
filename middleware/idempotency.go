package middleware

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

// ErrIdempotencyResponseLimit indicates that an opted-in handler exceeded its
// recording budget. The action remains recorded as failed and is never rerun.
var ErrIdempotencyResponseLimit = errors.New("idempotency response exceeds recording limit")

type IdempotencyOptions struct {
	MaxEntries       int
	TTL              time.Duration
	MaxBodyBytes     int64
	MaxResponseBytes int
	// MaxWaiters bounds concurrent duplicate waiters across all entries.
	// Zero uses MaxEntries. WaitTimeout zero uses TTL.
	MaxWaiters  int
	WaitTimeout time.Duration
	// Route is an explicit stable route identity, not a client-supplied path.
	Route string
	// Header defaults to Idempotency-Key. Values must be printable ASCII,
	// without spaces, and at most 128 bytes.
	Header string
	// ReplayHeaders opts additional application response headers into replay.
	// Transport, request metadata, cache, and security headers are rejected.
	ReplayHeaders []string
	// Scope must return an authenticated principal/session identity.
	// Never use an unvalidated client header as the scope.
	Scope func(*http.Request) (string, error)
	// Validate checks current replay access and input before reservation and
	// again immediately before a duplicate is replayed (including after waits).
	// It must be read-only and reentrant. Check current permissions here, not
	// only in a transition authorizer skipped by replay. Body reads are reset.
	// Do not put model-version preconditions here. Return httpx.Error for
	// public errors.
	Validate     func(*http.Request) error
	ErrorHandler httpx.ErrorHandler
}

type idempotencyEntry struct {
	fingerprint [32]byte
	done        chan struct{}
	expires     time.Time
	response    recordedResponse
	failed      bool
}

type recordedResponse struct {
	status int
	header http.Header
	body   []byte
}

// Idempotency explicitly opts mutation routes into bounded in-memory response
// recording and single-flight duplicate suppression. This is NOT durable:
// restarts, separate processes, and expiration lose action history. It provides
// no crash durability or exactly-once guarantee for external effects. Use a
// transactional durable action log when those guarantees are required.
//
// Apply authentication, CSRF, admission, and request metadata middleware outside
// this adapter. Streaming, informational responses, upgrades, trailers, and
// hijacking are unsupported. ResponseController cannot unwrap this intentional
// buffering boundary. A panic, unsupported response, or overflow records a
// terminal failure for the TTL, because the mutation may already have committed.
// Pending actions are never evicted; a full table rejects immediately.
//
// Completed responses replay representation headers, Location, and explicitly
// configured application headers. Current
// request IDs, security/CORS headers, and cookies remain current; Set-Cookie,
// Server-Timing, tracing metadata, and hop-by-hop headers are never replayed.
// All responses are no-store. Limits include response header bytes.
func Idempotency(options IdempotencyOptions) (Extension, error) {
	if options.MaxEntries <= 0 || options.TTL <= 0 || options.MaxBodyBytes <= 0 ||
		options.MaxBodyBytes == int64(^uint64(0)>>1) || options.MaxResponseBytes <= 0 ||
		options.Route == "" || len(options.Route) > 128 || options.Scope == nil {
		return nil, fmt.Errorf("idempotency: positive bounds, TTL, route and principal scope required")
	}
	if options.Header == "" {
		options.Header = "Idempotency-Key"
	}
	if !httpToken(options.Header) {
		return nil, fmt.Errorf("idempotency: invalid action header")
	}
	if options.MaxWaiters < 0 || options.WaitTimeout < 0 {
		return nil, fmt.Errorf("idempotency: invalid waiter bounds")
	}
	if options.MaxWaiters == 0 {
		options.MaxWaiters = options.MaxEntries
	}
	if options.WaitTimeout == 0 {
		options.WaitTimeout = options.TTL
	}
	replayHeaders := []string{"Content-Type", "Content-Language", "Content-Disposition", "Content-Encoding", "Location"}
	for _, header := range options.ReplayHeaders {
		if !httpToken(header) || unsafeReplayHeader(header) {
			return nil, fmt.Errorf("idempotency: unsafe replay header %q", header)
		}
		replayHeaders = append(replayHeaders, http.CanonicalHeaderKey(header))
	}
	fail := extensionError(options.ErrorHandler)
	entries := make(map[[32]byte]*idempotencyEntry)
	var mu sync.Mutex
	var nextExpiry time.Time
	waiters := 0
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch && r.Method != http.MethodDelete {
				w.Header().Set("Allow", "POST, PUT, PATCH, DELETE")
				rejectExtension(fail, w, r, http.StatusMethodNotAllowed)
				return
			}
			if r.Header.Get("Upgrade") != "" || headerToken(r.Header.Get("Connection"), "upgrade") {
				rejectExtension(fail, w, r, http.StatusBadRequest)
				return
			}
			action := r.Header.Get(options.Header)
			if len(r.Header.Values(options.Header)) != 1 || !validActionID(action) {
				rejectExtension(fail, w, r, http.StatusBadRequest)
				return
			}
			scope, err := options.Scope(r)
			if err != nil || scope == "" || len(scope) > 1024 {
				fail(w, r, &httpx.Error{Status: http.StatusUnauthorized, Cause: err})
				return
			}
			contentType := r.Header.Get("Content-Type")
			if len(r.Header.Values("Content-Type")) > 1 {
				rejectExtension(fail, w, r, http.StatusBadRequest)
				return
			}
			if len(r.Header.Values("Content-Encoding")) > 1 {
				rejectExtension(fail, w, r, http.StatusBadRequest)
				return
			}
			if contentType != "" {
				mediaType, parameters, err := mime.ParseMediaType(contentType)
				if err != nil {
					rejectExtension(fail, w, r, http.StatusBadRequest)
					return
				}
				contentType = mime.FormatMediaType(mediaType, parameters)
			}
			if r.ContentLength > options.MaxBodyBytes {
				rejectExtension(fail, w, r, http.StatusRequestEntityTooLarge)
				return
			}
			var body []byte
			if r.Body != nil {
				original := r.Body
				body, err = io.ReadAll(io.LimitReader(original, options.MaxBodyBytes+1))
				closeErr := original.Close()
				if err == nil {
					err = closeErr
				}
			}
			if err != nil {
				fail(w, r, &httpx.Error{Status: http.StatusBadRequest, Cause: err})
				return
			}
			if int64(len(body)) > options.MaxBodyBytes {
				rejectExtension(fail, w, r, http.StatusRequestEntityTooLarge)
				return
			}
			if err := r.Context().Err(); err != nil {
				fail(w, r, err)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			if options.Validate != nil {
				if err := options.Validate(r); err != nil {
					fail(w, r, err)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
			key := idempotencyHash([]byte(scope), []byte(action))
			fingerprint := idempotencyHash([]byte(r.Method), []byte(options.Route), []byte(r.URL.EscapedPath()),
				[]byte(r.URL.RawQuery), []byte(contentType),
				[]byte(r.Header.Get("Content-Encoding")), []byte(r.Header.Get("Accept")),
				[]byte(r.Header.Get("Accept-Language")), idempotencyHXHeaders(r.Header), body)
			now := time.Now()
			mu.Lock()
			// Pending entries have zero expires and cannot be evicted: doing so
			// would allow a still-running mutation to execute twice.
			if !nextExpiry.IsZero() && !now.Before(nextExpiry) {
				nextExpiry = time.Time{}
				for key, entry := range entries {
					if entry.expires.IsZero() {
						continue
					}
					if !now.Before(entry.expires) {
						delete(entries, key)
					} else if nextExpiry.IsZero() || entry.expires.Before(nextExpiry) {
						nextExpiry = entry.expires
					}
				}
			}
			entry, duplicate := entries[key]
			if duplicate && entry.fingerprint != fingerprint {
				mu.Unlock()
				rejectExtension(fail, w, r, http.StatusConflict)
				return
			}
			if !duplicate && len(entries) >= options.MaxEntries {
				mu.Unlock()
				httpx.Observe(r.Context(), httpx.IdempotencyCapacity)
				setRetryAfter(w, options.TTL)
				rejectExtension(fail, w, r, http.StatusServiceUnavailable)
				return
			}
			pending := duplicate && entry.expires.IsZero()
			if !duplicate {
				entry = &idempotencyEntry{fingerprint: fingerprint, done: make(chan struct{})}
				entries[key] = entry
			} else if pending {
				if waiters >= options.MaxWaiters {
					mu.Unlock()
					httpx.Observe(r.Context(), httpx.IdempotencyWaitCapacity)
					setRetryAfter(w, options.WaitTimeout)
					rejectExtension(fail, w, r, http.StatusServiceUnavailable)
					return
				}
				waiters++
			}
			mu.Unlock()
			if duplicate {
				if pending {
					defer func() {
						mu.Lock()
						waiters--
						mu.Unlock()
					}()
					timer := time.NewTimer(options.WaitTimeout)
					defer timer.Stop()
					select {
					case <-r.Context().Done():
						fail(w, r, r.Context().Err())
						return
					case <-timer.C:
						setRetryAfter(w, options.WaitTimeout)
						rejectExtension(fail, w, r, http.StatusServiceUnavailable)
						return
					case <-entry.done:
					}
				}
				// Permissions or session ownership may change while a duplicate
				// waits. Recheck immediately before exposing the recorded body.
				currentScope, err := options.Scope(r)
				if err != nil || currentScope != scope {
					fail(w, r, &httpx.Error{Status: http.StatusUnauthorized, Cause: err})
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				if options.Validate != nil {
					if err := options.Validate(r); err != nil {
						fail(w, r, err)
						return
					}
				}
				if entry.failed {
					rejectExtension(fail, w, r, http.StatusInternalServerError)
					return
				}
				replayIdempotency(w, r, entry.response, replayHeaders)
				return
			}
			capture := &idempotencyWriter{header: w.Header().Clone(), max: options.MaxResponseBytes}
			// Publish even when the handler panics. Recover middleware outside
			// this adapter still receives the original panic.
			finished := false
			defer func() {
				mu.Lock()
				entry.failed = !finished || capture.err != nil
				if !entry.failed {
					entry.response = capture.recorded()
					safe := make(http.Header)
					for _, key := range replayHeaders {
						if values, ok := entry.response.header[key]; ok {
							safe[key] = append([]string(nil), values...)
						}
					}
					entry.response.header = safe
				}
				entry.expires = time.Now().Add(options.TTL)
				if nextExpiry.IsZero() || entry.expires.Before(nextExpiry) {
					nextExpiry = entry.expires
				}
				close(entry.done)
				mu.Unlock()
			}()
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(capture, r)
			capture.validate()
			finished = true
			if capture.err != nil {
				fail(w, r, &httpx.Error{Status: http.StatusInternalServerError, Cause: capture.err})
				return
			}
			response := capture.recorded()
			for key, values := range response.header {
				w.Header()[key] = append([]string(nil), values...)
			}
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(response.status)
			if _, err := w.Write(response.body); err != nil {
				slog.ErrorContext(r.Context(), "idempotency response write failed", "error", err,
					"request_id", httpx.RequestID(r.Context()))
			}
		})
	}, nil
}

func validActionID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if char < 33 || char > 126 {
			return false
		}
	}
	return true
}

func idempotencyHash(parts ...[]byte) [32]byte {
	var digest hash.Hash = sha256.New()
	var size [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(size[:], uint64(len(part)))
		_, _ = digest.Write(size[:])
		_, _ = digest.Write(part)
	}
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func idempotencyHXHeaders(header http.Header) []byte {
	// Sort names while retaining individual field/value boundaries. Include
	// all HX fields, not only HX-Request: targets/history/boosting can change
	// the representation returned by the same mutation route.
	var names []string
	for name := range header {
		if strings.HasPrefix(strings.ToLower(name), "hx-") {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	var parts [][]byte
	for _, name := range names {
		field := [][]byte{[]byte(strings.ToLower(name))}
		for _, value := range header[name] {
			field = append(field, []byte(value))
		}
		sum := idempotencyHash(field...)
		parts = append(parts, sum[:])
	}
	sum := idempotencyHash(parts...)
	return sum[:]
}

func replayIdempotency(w http.ResponseWriter, r *http.Request, response recordedResponse, headers []string) {
	for _, key := range headers {
		if values, ok := response.header[key]; ok {
			w.Header()[key] = append([]string(nil), values...)
		}
	}
	w.Header().Del("Content-Length")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(response.status)
	if _, err := w.Write(response.body); err != nil {
		slog.ErrorContext(r.Context(), "idempotency replay write failed", "error", err,
			"request_id", httpx.RequestID(r.Context()))
	}
}

func unsafeReplayHeader(header string) bool {
	header = strings.ToLower(header)
	if strings.HasPrefix(header, "access-control-") || strings.HasPrefix(header, "sec-") ||
		strings.HasPrefix(header, "x-forwarded-") || strings.HasPrefix(header, "x-b3-") ||
		strings.HasPrefix(header, "x-envoy-") || strings.HasPrefix(header, strings.ToLower(http.TrailerPrefix)) {
		return true
	}
	switch header {
	case "set-cookie", "set-cookie2", "cookie", "server-timing", "timing-allow-origin",
		"traceparent", "tracestate", "baggage", "x-request-id", "x-correlation-id",
		"request-id", "correlation-id", "date", "server", "connection", "keep-alive",
		"proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding",
		"upgrade", "content-length", "cache-control", "pragma", "expires", "age", "vary",
		"etag", "last-modified", "strict-transport-security", "content-security-policy",
		"content-security-policy-report-only", "permissions-policy", "referrer-policy",
		"x-content-type-options", "x-frame-options", "x-xss-protection",
		"cross-origin-embedder-policy", "cross-origin-opener-policy", "cross-origin-resource-policy",
		"authorization", "www-authenticate":
		return true
	}
	return false
}

type idempotencyWriter struct {
	header      http.Header
	sent        http.Header
	status      int
	body        bytes.Buffer
	max         int
	headerBytes int
	err         error
}

func (w *idempotencyWriter) Header() http.Header { return w.header }

func (w *idempotencyWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	if status < 100 || status > 999 {
		panic("invalid HTTP status code")
	}
	if status < 200 {
		w.err = fmt.Errorf("idempotency: informational responses and upgrades unsupported")
	}
	w.status = status
	w.validate()
	if w.err == nil {
		w.sent = w.header.Clone()
	}
}

func (w *idempotencyWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.status == http.StatusNoContent || w.status == http.StatusNotModified || w.status == http.StatusResetContent {
		return 0, http.ErrBodyNotAllowed
	}
	if len(body) > w.max-w.headerBytes-w.body.Len() {
		w.err = ErrIdempotencyResponseLimit
	}
	if w.err != nil {
		return 0, w.err
	}
	return w.body.Write(body)
}

// FlushError intentionally prevents ResponseController from reaching an
// underlying transport. The failure becomes terminal even if ignored.
func (w *idempotencyWriter) FlushError() error {
	w.err = fmt.Errorf("idempotency: streaming unsupported: %w", http.ErrNotSupported)
	return w.err
}

func (w *idempotencyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.err = fmt.Errorf("idempotency: hijacking unsupported: %w", http.ErrNotSupported)
	return nil, nil, w.err
}

func (w *idempotencyWriter) EnableFullDuplex() error {
	return w.FlushError()
}

func (w *idempotencyWriter) validate() {
	headerBytes := 0
	for key, values := range w.header {
		if strings.EqualFold(key, "Trailer") || strings.HasPrefix(strings.ToLower(key), strings.ToLower(http.TrailerPrefix)) ||
			strings.EqualFold(key, "Upgrade") || strings.EqualFold(key, "Transfer-Encoding") ||
			strings.EqualFold(key, "Connection") && headerToken(strings.Join(values, ","), "upgrade") {
			w.err = fmt.Errorf("idempotency: trailers and upgrades unsupported")
			return
		}
		for _, value := range values {
			headerBytes += len(key) + len(value) + 4
			if headerBytes > w.max-w.body.Len() {
				w.err = ErrIdempotencyResponseLimit
				return
			}
		}
		w.headerBytes = headerBytes
	}
}

func (w *idempotencyWriter) recorded() recordedResponse {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return recordedResponse{status: w.status, header: w.sent.Clone(), body: append([]byte(nil), w.body.Bytes()...)}
}

// Ensure the adapter has no accidental unwrap path.
var _ interface{ FlushError() error } = (*idempotencyWriter)(nil)
