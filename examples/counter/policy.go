package main

import (
	"context"
	"net/http"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

// Keep the starter copyable without importing repository-internal helpers.
func requestLimits(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.Context().Err(); err != nil {
			httpx.RespondError(w, r, err)
			return
		}
		const maxBodyBytes = 1 << 20
		if r.ContentLength > maxBodyBytes {
			httpx.RespondError(w, r, &httpx.Error{Status: http.StatusRequestEntityTooLarge})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		r = r.WithContext(ctx)
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		tracked := &httpx.Writer{ResponseWriter: w}
		next.ServeHTTP(tracked, r)
		if err := ctx.Err(); err != nil && !tracked.Started {
			httpx.RespondError(w, r, err)
		}
	})
}
