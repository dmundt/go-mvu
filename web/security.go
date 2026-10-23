package web

import (
	"fmt"
	"log/slog"
	"net/http"
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

// Recover converts uncommitted panics into 500 responses. Committed responses
// are aborted, never replaced or appended to.
func Recover(next http.Handler) http.Handler {
	return RecoverWith(RespondError)(next)
}

func RecoverWith(onError ErrorHandler) func(http.Handler) http.Handler {
	if onError == nil {
		onError = RespondError
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tracked := &ResponseWriter{ResponseWriter: w}
			defer func() {
				if value := recover(); value != nil {
					if value == http.ErrAbortHandler {
						panic(value)
					}
					slog.ErrorContext(r.Context(), "panic", "method", r.Method,
						"path", r.URL.Path, "request_id", RequestID(r.Context()),
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
