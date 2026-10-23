package web

import (
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/store"
)

type OutcomeState uint8

const (
	OutcomeRejected OutcomeState = iota + 1
	OutcomeCompleted
	OutcomeCommitted
	OutcomeEffectFailed
	OutcomeResponseAborted
)

// RequestOutcome describes storage and response state when request handling ends.
type RequestOutcome struct {
	State     OutcomeState
	Committed bool
	Err       error
}

type OutcomeObserver func(*http.Request, RequestOutcome)

// Handle decodes a message, applies Update, runs commands, renders Result.
func (runtime *Runtime[K, Model, Msg]) Handle(route Route[Msg]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracked := &httpx.Writer{ResponseWriter: w}
		var committed atomic.Bool
		var outcomeErr error
		defer func() {
			panicValue := recover()
			runtime.notifyOutcome(r, tracked, committed.Load(), outcomeErr, panicValue)
			if panicValue != nil {
				panic(panicValue)
			}
		}()
		fail := func(err error) {
			outcomeErr = err
			runtime.fail(tracked, r, err)
		}
		if route.Pattern != "" && r.URL.Path != route.Pattern {
			fail(&httpx.Error{Status: http.StatusNotFound, Public: http.StatusText(http.StatusNotFound)})
			return
		}
		if route.Method != "" && r.Method != route.Method {
			tracked.Header().Set("Allow", route.Method)
			fail(&httpx.Error{Status: http.StatusMethodNotAllowed, Public: http.StatusText(http.StatusMethodNotAllowed)})
			return
		}
		if route.Decode == nil {
			fail(ErrInvalidRuntime)
			return
		}
		key, err := runtime.keys(r)
		if err != nil {
			fail(err)
			return
		}
		ctx := r.Context()
		if err := ctx.Err(); err != nil {
			fail(err)
			return
		}
		var expected *store.Version
		if route.ExpectedVersion != nil {
			version, err := route.ExpectedVersion(r)
			if err != nil {
				fail(err)
				return
			}
			expected = &version
		}
		trace := httpx.TraceFromContext(ctx)
		if route.RequireModel {
			if err := ctx.Err(); err != nil {
				fail(err)
				return
			}
			done := trace.Start(httpx.PhaseLoad)
			_, err := runtime.store.Load(ctx, key)
			done()
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					fail(&httpx.Error{Status: http.StatusNotFound, Public: http.StatusText(http.StatusNotFound), Cause: err})
				} else {
					fail(err)
				}
				return
			}
		}
		if err := ctx.Err(); err != nil {
			fail(err)
			return
		}
		done := trace.Start(httpx.PhaseDecode)
		message, err := route.Decode(r)
		done()
		if err != nil {
			fail(err)
			return
		}
		if err := ctx.Err(); err != nil {
			fail(err)
			return
		}
		done = trace.Start(httpx.PhaseDispatch)
		err = runtime.dispatcher(key, expected, func() { committed.Store(true) })(ctx, message)
		done()
		if err != nil {
			fail(err)
			return
		}
		if err := ctx.Err(); err != nil {
			fail(err)
			return
		}
		done = trace.Start(httpx.PhaseLoad)
		snapshot, err := runtime.store.Load(ctx, key)
		done()
		if err != nil {
			fail(err)
			return
		}
		outcomeErr = runtime.writeResult(tracked, r, snapshot.Model, route.Result)
	})
}

// Page loads or creates the model and renders result without Update.
func (runtime *Runtime[K, Model, Msg]) Page(result Result) http.Handler {
	return runtime.PageFunc(func(*http.Request) Result { return result })
}

// PageFunc is Page with a per-request result (for example a fragment name
// chosen from the query).
func (runtime *Runtime[K, Model, Msg]) PageFunc(result func(*http.Request) Result) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracked := &httpx.Writer{ResponseWriter: w}
		var committed atomic.Bool
		var outcomeErr error
		defer func() {
			panicValue := recover()
			runtime.notifyOutcome(r, tracked, committed.Load(), outcomeErr, panicValue)
			if panicValue != nil {
				panic(panicValue)
			}
		}()
		fail := func(err error) {
			outcomeErr = err
			runtime.fail(tracked, r, err)
		}
		if result == nil {
			fail(ErrInvalidRuntime)
			return
		}
		key, err := runtime.keys(r)
		if err != nil {
			fail(err)
			return
		}
		if err := r.Context().Err(); err != nil {
			fail(err)
			return
		}
		trace := httpx.TraceFromContext(r.Context())
		done := trace.Start(httpx.PhaseLoad)
		snapshot, commands, created, err := runtime.loadOrCreate(r.Context(), key)
		done()
		if err != nil {
			fail(err)
			return
		}
		committed.Store(created)
		if len(commands) > 0 {
			done = trace.Start(httpx.PhaseDispatch)
			err = runtime.commands.Run(r.Context(), commands,
				runtime.dispatcher(key, nil, func() { committed.Store(true) }))
			done()
			if err != nil {
				if committed.Load() && !isEffectError(err) {
					err = &EffectError{Cause: err}
				}
				fail(err)
				return
			}
			done = trace.Start(httpx.PhaseLoad)
			snapshot, err = runtime.store.Load(r.Context(), key)
			done()
			if err != nil {
				fail(err)
				return
			}
		}
		outcomeErr = runtime.writeResult(tracked, r, snapshot.Model, result(r))
	})
}

func (runtime *Runtime[K, Model, Msg]) notifyOutcome(
	r *http.Request, tracked *httpx.Writer, committed bool, err error, panicValue any,
) {
	state := OutcomeRejected
	switch {
	case panicValue != nil && tracked.Started:
		state = OutcomeResponseAborted
	case isEffectError(err):
		state = OutcomeEffectFailed
		httpx.Observe(r.Context(), httpx.CommittedEffectFailure)
	case isResponseAbort(err):
		state = OutcomeResponseAborted
		httpx.Observe(r.Context(), httpx.ResponseAborted)
	case committed:
		state = OutcomeCommitted
	case err == nil && panicValue == nil:
		state = OutcomeCompleted
	}
	if panicValue != nil && err == nil {
		err = fmt.Errorf("request panic: %v", panicValue)
	}
	if runtime.observe != nil {
		runtime.observe(r, RequestOutcome{State: state, Committed: committed, Err: err})
	}
}
