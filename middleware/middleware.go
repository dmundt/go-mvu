package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
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

// Recover converts panics into 500 responses and logs them.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				slog.ErrorContext(r.Context(), "panic", "method", r.Method,
					"path", r.URL.Path, "request_id", RequestIDFromContext(r.Context()),
					"panic", value)
				http.Error(w, http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type requestIDKey struct{}

// RequestIDHeader carries the per-request identifier in responses.
const RequestIDHeader = "X-Request-ID"

// RequestID assigns a random identifier to each request, stores it in the
// request context, and echoes it in the X-Request-ID response header.
// Incoming IDs are ignored so clients cannot inject log content.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw [16]byte
		_, _ = rand.Read(raw[:])
		id := hex.EncodeToString(raw[:])
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

// RequestIDFromContext returns the ID assigned by RequestID, or "".
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach Flush and deadlines.
func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// RequestLogger logs one structured record per request. nil uses
// slog.Default().
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", recorder.bytes),
				slog.Duration("duration", time.Since(started)),
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.Bool("htmx", r.Header.Get("HX-Request") == "true"),
			)
		})
	}
}
