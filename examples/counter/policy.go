package main

import (
	"context"
	"net/http"
	"time"

	"github.com/dmundt/go-mvu/web"
)

// Keep the starter copyable without importing repository-internal helpers.
func requestLimits(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.Context().Err(); err != nil {
			web.RespondError(w, r, err)
			return
		}
		const maxBodyBytes = 1 << 20
		if r.ContentLength > maxBodyBytes {
			web.RespondError(w, r, &web.Error{Status: http.StatusRequestEntityTooLarge})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		r = r.WithContext(ctx)
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		tracked := &web.ResponseWriter{ResponseWriter: w}
		next.ServeHTTP(tracked, r)
		if err := ctx.Err(); err != nil && !tracked.Started {
			web.RespondError(w, r, err)
		}
	})
}
