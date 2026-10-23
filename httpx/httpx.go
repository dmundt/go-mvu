// Package httpx contains shared, transport-only HTTP contracts.
package httpx

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// Error carries a safe public status and an internal cause.
// Location, when set on a 401, requests a login navigation.
type Error struct {
	Status   int
	Public   string
	Location string
	Cause    error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return http.StatusText(e.Status)
}

func (e *Error) Unwrap() error { return e.Cause }

type ErrorHandler func(http.ResponseWriter, *http.Request, error)

type requestIDKey struct{}
type localeKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// WithLocale carries an explicitly resolved locale across HTTP package
// boundaries. Locale selection and validation remain middleware policy.
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeKey{}, locale)
}

func Locale(ctx context.Context) string {
	locale, _ := ctx.Value(localeKey{}).(string)
	return locale
}

// RespondError is the default rejection policy. Causes are logged, not exposed.
// HTMX error bodies retain their HTTP status; swapping requires explicit client
// responseHandling configuration. Login redirects navigate the whole document.
func RespondError(w http.ResponseWriter, r *http.Request, err error) {
	status, public, location := http.StatusInternalServerError, "", ""
	var failure *Error
	if errors.As(err, &failure) && failure.Status >= 400 && failure.Status <= 599 {
		status, public, location = failure.Status, failure.Public, failure.Location
	} else if errors.Is(err, context.DeadlineExceeded) {
		status = http.StatusGatewayTimeout
	} else if errors.Is(err, context.Canceled) {
		status = http.StatusRequestTimeout
	}
	slog.ErrorContext(r.Context(), "HTTP request failed",
		"status", status, "request_id", RequestID(r.Context()), "error", err)
	w.Header().Set("Cache-Control", "no-store")
	if status == http.StatusUnauthorized && safeLocation(location) {
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", location)
		} else {
			http.Redirect(w, r, location, http.StatusSeeOther)
			return
		}
	}
	if public == "" {
		public = http.StatusText(status)
	}
	if id := RequestID(r.Context()); id != "" {
		public += "\nRequest ID: " + id
	}
	http.Error(w, public, status)
}

func safeLocation(location string) bool {
	return strings.HasPrefix(location, "/") && !strings.HasPrefix(location, "//") &&
		!strings.ContainsAny(location, "\\\x00\r\n\t") &&
		strings.IndexFunc(location, func(r rune) bool { return r < 32 || r == 127 }) < 0
}

// Vary merges field names without discarding previously configured variations.
func Vary(header http.Header, fields ...string) {
	for _, field := range fields {
		found := false
		for _, line := range header.Values("Vary") {
			for _, existing := range strings.Split(line, ",") {
				if strings.TrimSpace(existing) == "*" {
					return
				}
				if strings.EqualFold(strings.TrimSpace(existing), field) {
					found = true
				}
			}
		}
		if !found {
			header.Add("Vary", field)
		}
	}
}

// Writer tracks final response commitment and bytes accepted by the underlying
// writer. Informational responses other than 101 do not commit the final status.
// Use http.ResponseController for flushing and deadlines.
type Writer struct {
	http.ResponseWriter
	Status  int
	Bytes   int
	Started bool
}

func (w *Writer) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *Writer) WriteHeader(status int) {
	if w.Started {
		return
	}
	if status < 100 || status > 999 {
		panic("invalid HTTP status code")
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.Status, w.Started = status, true
	w.ResponseWriter.WriteHeader(status)
}

func (w *Writer) Write(data []byte) (int, error) {
	if !w.Started {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.Bytes += n
	return n, err
}

func (w *Writer) FlushError() error {
	if !w.Started {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *Writer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, buffer, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.Started = true
		if w.Status == 0 {
			w.Status = http.StatusSwitchingProtocols
		}
	}
	return conn, buffer, err
}
