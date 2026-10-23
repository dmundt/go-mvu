package middleware

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

type traceContextKey struct{}

// TraceContext is a validated W3C version-00 traceparent. Tracestate and baggage
// are deliberately not exposed or propagated by this adapter.
type TraceContext struct {
	TraceID  string
	ParentID string
	Sampled  bool
}

func TraceFromContext(ctx context.Context) (TraceContext, bool) {
	trace, ok := ctx.Value(traceContextKey{}).(TraceContext)
	return trace, ok
}

type Completion struct {
	Route     string
	RequestID string
	Status    int
	Bytes     int
	Duration  time.Duration
	Panic     any
	Err       error
}

type TelemetryOptions struct {
	// Route is a fixed, low-cardinality name, never a raw URL or user input.
	// Construct the adapter per route (or explicitly named route group).
	Route    string
	Complete func(context.Context, Completion)
}

// Telemetry calls Complete exactly once, including cancellation and panic.
// Callbacks must not panic. Panics from downstream handlers are rethrown.
func Telemetry(options TelemetryOptions) (Extension, error) {
	if options.Route == "" || len(options.Route) > 128 || strings.ContainsAny(options.Route, "\r\n") || options.Complete == nil {
		return nil, fmt.Errorf("telemetry: fixed route name and completion callback required")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if values := r.Header.Values("Traceparent"); len(values) == 1 {
				if trace, ok := parseTraceparent(values[0]); ok {
					r = r.WithContext(context.WithValue(r.Context(), traceContextKey{}, trace))
				}
			}
			start := time.Now()
			writer := &httpx.Writer{ResponseWriter: w}
			defer func() {
				value := recover()
				status := writer.Status
				if status == 0 {
					status = http.StatusOK
					if value != nil {
						status = http.StatusInternalServerError
					}
				}
				options.Complete(r.Context(), Completion{
					Route: options.Route, RequestID: httpx.RequestID(r.Context()),
					Status: status, Bytes: writer.Bytes, Duration: time.Since(start),
					Panic: value, Err: r.Context().Err(),
				})
				if value != nil {
					panic(value)
				}
			}()
			next.ServeHTTP(writer, r)
		})
	}, nil
}

func parseTraceparent(value string) (TraceContext, bool) {
	if len(value) != 55 || value[:3] != "00-" || value[35] != '-' || value[52] != '-' || strings.ToLower(value) != value {
		return TraceContext{}, false
	}
	trace, parent, flags := value[3:35], value[36:52], value[53:]
	for _, part := range []string{trace, parent, flags} {
		if _, err := hex.DecodeString(part); err != nil {
			return TraceContext{}, false
		}
	}
	if trace == strings.Repeat("0", 32) || parent == strings.Repeat("0", 16) {
		return TraceContext{}, false
	}
	flag, _ := hex.DecodeString(flags)
	return TraceContext{TraceID: trace, ParentID: parent, Sampled: flag[0]&1 != 0}, true
}
