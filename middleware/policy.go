package middleware

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

// RequestPolicyOptions configures transport limits. A zero body limit uses
// 1 MiB. A zero timeout leaves the server's request context unchanged.
type RequestPolicyOptions struct {
	MaxBodyBytes int64
	Timeout      time.Duration
	// ContentTypes restricts unsafe requests with a body. Empty accepts all.
	// Apply a route-specific policy when different routes accept different types.
	ContentTypes []string
	OnError      httpx.ErrorHandler
}

// NewRequestPolicy bounds body reads before any parser, including requests
// carrying CSRF in a header. Decoders must propagate body read errors.
// Deadlines are cooperative: this never races a handler to write a timeout.
func NewRequestPolicy(options RequestPolicyOptions) (func(http.Handler) http.Handler, error) {
	if options.MaxBodyBytes < 0 || options.Timeout < 0 {
		return nil, errors.New("request limits must not be negative")
	}
	if options.MaxBodyBytes == 0 {
		options.MaxBodyBytes = maxFormBytes
	}
	types := make(map[string]bool, len(options.ContentTypes))
	for _, contentType := range options.ContentTypes {
		parsed, params, err := mime.ParseMediaType(contentType)
		major, minor, hasSlash := strings.Cut(parsed, "/")
		if err != nil || len(params) != 0 || !hasSlash || major == "" || minor == "" ||
			strings.Contains(parsed, "*") {
			return nil, fmt.Errorf("invalid request content type %q", contentType)
		}
		types[parsed] = true
	}
	onError := options.OnError
	if onError == nil {
		onError = httpx.RespondError
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.Context().Err(); err != nil {
				onError(w, r, err)
				return
			}
			if r.ContentLength > options.MaxBodyBytes {
				onError(w, r, &httpx.Error{Status: http.StatusRequestEntityTooLarge})
				return
			}
			if unsafeMethod(r.Method) && r.Body != nil && r.Body != http.NoBody && len(types) != 0 {
				contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
				if err != nil || !types[contentType] {
					onError(w, r, &httpx.Error{Status: http.StatusUnsupportedMediaType, Cause: err})
					return
				}
			}
			if options.Timeout > 0 {
				ctx, cancel := context.WithTimeout(r.Context(), options.Timeout)
				defer cancel()
				r = r.WithContext(ctx)
			}
			r.Body = http.MaxBytesReader(w, r.Body, options.MaxBodyBytes)
			tracked := &httpx.Writer{ResponseWriter: w}
			next.ServeHTTP(tracked, r)
			if err := r.Context().Err(); err != nil && !tracked.Started {
				onError(w, r, err)
			}
		})
	}, nil
}

// NoStore is the default for session-bearing pages. Static assets should be
// mounted outside this wrapper to keep their independent caching policy.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
