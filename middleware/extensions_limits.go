package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

type AdmissionOptions struct {
	MaxConcurrent int
	RetryAfter    time.Duration
	ErrorHandler  httpx.ErrorHandler
}

// AdmissionLimit rejects immediately rather than creating a waiting queue.
func AdmissionLimit(options AdmissionOptions) (Extension, error) {
	if options.MaxConcurrent <= 0 || options.RetryAfter <= 0 {
		return nil, fmt.Errorf("admission: positive concurrency and retry delay required")
	}
	slots := make(chan struct{}, options.MaxConcurrent)
	fail := extensionError(options.ErrorHandler)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
				next.ServeHTTP(w, r)
			default:
				httpx.Observe(r.Context(), httpx.AdmissionOverload)
				setRetryAfter(w, options.RetryAfter)
				rejectExtension(fail, w, r, http.StatusServiceUnavailable)
			}
		})
	}, nil
}

func setRetryAfter(w http.ResponseWriter, duration time.Duration) {
	w.Header().Set("Retry-After", strconv.FormatInt(int64(math.Ceil(duration.Seconds())), 10))
}

type RateLimitOptions struct {
	MaxEntries int
	Requests   int
	Window     time.Duration
	// Key must supply an application-selected identity. RemoteAddr and
	// forwarding headers are not implicitly trusted. Empty/oversize keys fail.
	Key          func(*http.Request) (string, error)
	ErrorHandler httpx.ErrorHandler
}

type rateEntry struct {
	expires time.Time
	count   int
}

// RateLimit is a bounded, fixed-window, per-key limiter. Expired entries are
// reclaimed lazily, without background goroutines. Full tables reject new keys.
func RateLimit(options RateLimitOptions) (Extension, error) {
	if options.MaxEntries <= 0 || options.Requests <= 0 || options.Window <= 0 || options.Key == nil {
		return nil, fmt.Errorf("rate limit: positive bounds, window and key required")
	}
	var mu sync.Mutex
	entries := make(map[string]rateEntry)
	fail := extensionError(options.ErrorHandler)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, err := options.Key(r)
			if err != nil || len(key) == 0 || len(key) > 1024 {
				rejectExtension(fail, w, r, http.StatusBadRequest)
				return
			}
			now := time.Now()
			mu.Lock()
			entry, exists := entries[key]
			if exists && !now.Before(entry.expires) {
				delete(entries, key)
				exists = false
			}
			if !exists && len(entries) >= options.MaxEntries {
				for key, value := range entries {
					if !now.Before(value.expires) {
						delete(entries, key)
					}
				}
			}
			status, retry := 0, options.Window
			if !exists && len(entries) >= options.MaxEntries {
				status = http.StatusServiceUnavailable
			} else if exists && entry.count >= options.Requests {
				status, retry = http.StatusTooManyRequests, entry.expires.Sub(now)
			} else {
				if !exists {
					entry.expires = now.Add(options.Window)
				}
				entry.count++
				entries[key] = entry
			}
			mu.Unlock()
			if status != 0 {
				if status == http.StatusTooManyRequests {
					httpx.Observe(r.Context(), httpx.RateLimited)
				} else {
					httpx.Observe(r.Context(), httpx.RateCapacity)
				}
				setRetryAfter(w, retry)
				rejectExtension(fail, w, r, status)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}
