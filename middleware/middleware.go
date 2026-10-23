package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

// DefaultContentSecurityPolicy allows scripts only from the application
// origin (HTMX served from /static/). style-src permits 'unsafe-inline'
// because theme CSS is inlined in a <style> element; scripts never are.
const DefaultContentSecurityPolicy = "default-src 'self'; script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; " +
	"form-action 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'"

// DefaultPermissionsPolicy disables powerful browser features.
const DefaultPermissionsPolicy = "camera=(), microphone=(), geolocation=(), payment=(), usb=()"

// SecurityHeaders sets the default CSP, Permissions-Policy, MIME sniffing,
// referrer, and legacy frame protection headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", DefaultContentSecurityPolicy)
		header.Set("Permissions-Policy", DefaultPermissionsPolicy)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		header.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// Chain applies middleware in request order: first listed is outermost.
func Chain(handler http.Handler, wrappers ...func(http.Handler) http.Handler) http.Handler {
	for i := len(wrappers) - 1; i >= 0; i-- {
		handler = wrappers[i](handler)
	}
	return handler
}

// Recover converts uncommitted panics into 500 responses. Committed responses
// are aborted, never replaced or appended to.
func Recover(next http.Handler) http.Handler {
	return RecoverWith(httpx.RespondError)(next)
}

func RecoverWith(onError httpx.ErrorHandler) func(http.Handler) http.Handler {
	if onError == nil {
		onError = httpx.RespondError
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tracked := &httpx.Writer{ResponseWriter: w}
			defer func() {
				if value := recover(); value != nil {
					if value == http.ErrAbortHandler {
						panic(value)
					}
					slog.ErrorContext(r.Context(), "panic", "method", r.Method,
						"path", r.URL.Path, "request_id", RequestIDFromContext(r.Context()),
						"panic", value)
					if tracked.Started {
						panic(http.ErrAbortHandler)
					}
					onError(w, r, fmt.Errorf("handler panic: %v", value))
				}
			}()
			next.ServeHTTP(tracked, r)
		})
	}
}

// RequestIDHeader carries the per-request identifier in responses.
const RequestIDHeader = "X-Request-ID"

// RequestID assigns a random identifier to each request, stores it in the
// request context, and echoes it in the X-Request-ID response header.
// Incoming IDs are ignored so clients cannot inject log content.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			httpx.RespondError(w, r, err)
			return
		}
		id := hex.EncodeToString(raw[:])
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(httpx.WithRequestID(r.Context(), id)))
	})
}

// RequestIDFromContext returns the ID assigned by RequestID, or "".
func RequestIDFromContext(ctx context.Context) string {
	return httpx.RequestID(ctx)
}

// RequestLogger logs one structured record per request. nil uses
// slog.Default().
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &httpx.Writer{ResponseWriter: w}
			completed := false
			defer func() {
				status := recorder.Status
				if status == 0 {
					status = http.StatusOK
					if !completed {
						status = http.StatusInternalServerError
					}
				}
				logger.LogAttrs(r.Context(), slog.LevelInfo, "request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.Int("bytes", recorder.Bytes),
					slog.Bool("aborted", !completed),
					slog.Duration("duration", time.Since(started)),
					slog.String("request_id", RequestIDFromContext(r.Context())),
					slog.Bool("htmx", r.Header.Get("HX-Request") == "true"),
				)
			}()
			next.ServeHTTP(recorder, r)
			completed = true
		})
	}
}
