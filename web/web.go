// Package web binds MVU applications to net/http.
//
// Request flow: resolve key → decode message → load or create model →
// Update with compare-and-swap retry → run commands outside the store →
// render page or fragment with a RequestContext.
package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/store"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/timing"
)

var (
	ErrUnknownFragment            = errors.New("unknown HTML fragment")
	ErrInvalidRuntime             = errors.New("incomplete web runtime configuration")
	ErrUnknownRender              = errors.New("unknown render mode")
	ErrCommandOverload            = errors.New("async command queue full")
	ErrCommandClosed              = errors.New("async command runner closed")
	ErrInvalidCommandRunnerConfig = errors.New("invalid async command runner configuration")
)

// EffectError marks failures from commands after a model transition committed.
type EffectError struct{ Cause error }

func (e *EffectError) Error() string {
	if e.Cause == nil {
		return "command effect failed after commit"
	}
	return "command effect failed after commit: " + e.Cause.Error()
}
func (e *EffectError) Unwrap() error { return e.Cause }

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

// RequestContext carries per-request presentation state into views.
type RequestContext struct {
	Theme      theme.Theme
	Mode       theme.Mode
	IsHTMX     bool
	HTMX       htmx.Request
	CurrentURL string
	// Locale is explicitly resolved by HTTP middleware and passed to views.
	Locale string
}

// ThemeResolver reads and stores the visitor's color mode.
// theme.Preference implements it.
type ThemeResolver interface {
	Resolve(*http.Request) (theme.Mode, error)
	Persist(http.ResponseWriter, *http.Request, theme.Mode) error
}

// NewRequestContext builds view state. Nil theme → theme.DefaultTheme.
// Nil resolver or resolver error → mode from request context (System default).
func NewRequestContext(r *http.Request, active theme.Theme, resolver ThemeResolver) RequestContext {
	if active == nil {
		active = theme.DefaultTheme{}
	}
	mode := theme.ModeFromContext(r.Context())
	if resolver != nil {
		if resolved, err := resolver.Resolve(r); err == nil {
			mode = resolved
		}
	}
	request := htmx.FromRequest(r)
	current := r.URL.RequestURI()
	if request.IsHTMX && request.CurrentURL != "" {
		current = request.CurrentURL
	}
	return RequestContext{
		Theme: active, Mode: mode, IsHTMX: request.IsHTMX,
		HTMX: request, CurrentURL: current, Locale: httpx.Locale(r.Context()),
	}
}

type View[Model any] interface {
	Page(context.Context, Model, RequestContext) (html.Page, error)
	Fragment(context.Context, string, Model, RequestContext) (html.Fragment, error)
}

type ViewFunctions[Model any] struct {
	PageFunc     func(context.Context, Model, RequestContext) (html.Page, error)
	FragmentFunc func(context.Context, string, Model, RequestContext) (html.Fragment, error)
}

func (f ViewFunctions[Model]) Page(
	ctx context.Context, model Model, request RequestContext,
) (html.Page, error) {
	if f.PageFunc == nil {
		return nil, ErrInvalidRuntime
	}
	return f.PageFunc(ctx, model, request)
}

func (f ViewFunctions[Model]) Fragment(
	ctx context.Context, name string, model Model, request RequestContext,
) (html.Fragment, error) {
	if f.FragmentFunc == nil {
		return nil, ErrUnknownFragment
	}
	return f.FragmentFunc(ctx, name, model, request)
}

type Decoder[Msg mvu.Message] interface {
	Decode(*http.Request) (Msg, error)
}

type DecoderFunc[Msg mvu.Message] func(*http.Request) (Msg, error)

func (fn DecoderFunc[Msg]) Decode(r *http.Request) (Msg, error) { return fn(r) }

type StaticMessage[Msg mvu.Message] struct {
	Message Msg
}

func (s StaticMessage[Msg]) Decode(*http.Request) (Msg, error) { return s.Message, nil }

type KeyResolver[K comparable] interface {
	Resolve(*http.Request) (K, error)
}

type KeyResolverFunc[K comparable] func(*http.Request) (K, error)

func (fn KeyResolverFunc[K]) Resolve(r *http.Request) (K, error) { return fn(r) }

type ExpectedVersionResolver interface {
	ResolveVersion(*http.Request) (store.Version, error)
}

type ExpectedVersionFunc func(*http.Request) (store.Version, error)

func (fn ExpectedVersionFunc) ResolveVersion(r *http.Request) (store.Version, error) {
	return fn(r)
}

// CommandRunner executes commands returned by Init and Update.
// dispatch performs a full transition and runs follow-up commands.
type CommandRunner[Msg mvu.Message] interface {
	Run(context.Context, []mvu.Command[Msg], mvu.Dispatch[Msg]) error
}

// SyncCommands runs commands in order before the response renders.
type SyncCommands[Msg mvu.Message] struct{}

func (SyncCommands[Msg]) Run(
	ctx context.Context, commands []mvu.Command[Msg], dispatch mvu.Dispatch[Msg],
) error {
	for _, command := range commands {
		if command == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := command.Execute(ctx, dispatch); err != nil {
			return err
		}
	}
	return nil
}

// Deprecated: AsyncCommands creates unbounded goroutines. Use
// AsyncCommandRunner for bounded concurrency, queueing, and shutdown.
type AsyncCommands[Msg mvu.Message] struct {
	OnError func(error)
}

func (runner AsyncCommands[Msg]) Run(
	ctx context.Context, commands []mvu.Command[Msg], dispatch mvu.Dispatch[Msg],
) error {
	detached := context.WithoutCancel(ctx)
	for _, command := range commands {
		if command == nil {
			continue
		}
		go func() {
			if err := command.Execute(detached, dispatch); err != nil {
				runner.report(err)
			}
		}()
	}
	return nil
}

func (runner AsyncCommands[Msg]) report(err error) {
	if runner.OnError != nil {
		runner.OnError(err)
		return
	}
	slog.Error("mvu async command failed", "error", err)
}

type asyncTask[Msg mvu.Message] struct {
	command  mvu.Command[Msg]
	dispatch mvu.Dispatch[Msg]
	ctx      context.Context
}

// AsyncCommandRunner bounds concurrent command execution and queued work.
// Jobs retain request context values but not cancellation or deadlines.
// Close stops acceptance and drains queued jobs; a Close context timeout only
// bounds waiting and does not cancel running work. Per-command timeout bounds
// cooperative jobs independently of request and shutdown contexts.
type AsyncCommandRunner[Msg mvu.Message] struct {
	queue   chan asyncTask[Msg]
	timeout time.Duration
	onError func(error)
	mu      sync.Mutex
	closed  bool
	workers sync.WaitGroup
}

func NewAsyncCommandRunner[Msg mvu.Message](
	concurrency, queueSize int, timeout time.Duration, onError func(error),
) (*AsyncCommandRunner[Msg], error) {
	if concurrency <= 0 || queueSize < 0 || timeout <= 0 {
		return nil, ErrInvalidCommandRunnerConfig
	}
	runner := &AsyncCommandRunner[Msg]{
		queue: make(chan asyncTask[Msg], queueSize), timeout: timeout, onError: onError,
	}
	runner.workers.Add(concurrency)
	for range concurrency {
		go runner.worker()
	}
	return runner, nil
}

func (runner *AsyncCommandRunner[Msg]) Run(
	ctx context.Context, commands []mvu.Command[Msg], dispatch mvu.Dispatch[Msg],
) error {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.closed {
		return ErrCommandClosed
	}
	count := 0
	for _, command := range commands {
		if command != nil {
			count++
		}
	}
	if count > cap(runner.queue)-len(runner.queue) {
		if cap(runner.queue) == 0 && count == 1 {
			for _, command := range commands {
				if command == nil {
					continue
				}
				select {
				case runner.queue <- asyncTask[Msg]{
					command: command, dispatch: dispatch, ctx: context.WithoutCancel(ctx),
				}:
					return nil
				default:
					return ErrCommandOverload
				}
			}
		}
		return ErrCommandOverload
	}
	for _, command := range commands {
		if command == nil {
			continue
		}
		runner.queue <- asyncTask[Msg]{
			command: command, dispatch: dispatch, ctx: context.WithoutCancel(ctx),
		}
	}
	return nil
}

func (runner *AsyncCommandRunner[Msg]) worker() {
	defer runner.workers.Done()
	for task := range runner.queue {
		ctx, cancel := context.WithTimeout(task.ctx, runner.timeout)
		err := task.command.Execute(ctx, task.dispatch)
		cancel()
		if err != nil {
			runner.report(&EffectError{Cause: err})
		}
	}
}

func (runner *AsyncCommandRunner[Msg]) report(err error) {
	if runner.onError != nil {
		runner.onError(err)
		return
	}
	slog.Error("mvu async command failed", "error", err)
}

func (runner *AsyncCommandRunner[Msg]) Close(ctx context.Context) error {
	runner.mu.Lock()
	if !runner.closed {
		runner.closed = true
		close(runner.queue)
	}
	runner.mu.Unlock()
	done := make(chan struct{})
	go func() { runner.workers.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type RenderMode uint8

const (
	// RenderPage renders the full document.
	RenderPage RenderMode = iota + 1
	// RenderFragment renders Result.Fragment.
	RenderFragment
	// RenderNothing writes 204.
	RenderNothing
	// RenderAuto: HTMX (non-boosted, non-history) → fragment;
	// else Location set → redirect; else full page.
	RenderAuto
)

type Result struct {
	Mode       RenderMode
	Fragment   string
	Status     int
	Location   string
	PushURL    string
	ReplaceURL string
	// CacheControl overrides the default no-store policy for this result.
	// Set explicitly only when response content is safe to cache.
	CacheControl string
}

func (result Result) StatusOr(fallback int) int {
	if result.Status == 0 {
		return fallback
	}
	return result.Status
}

type Route[Msg mvu.Message] struct {
	Method  string
	Pattern string
	Decode  Decoder[Msg]
	Result  Result
	// RequireModel disables Init for this route: a missing model yields 404
	// instead of being created.
	RequireModel bool
	// ExpectedVersion is trusted application parsing of an intent version.
	// A stale intent is rejected with 409, never retried against newer state.
	ExpectedVersion ExpectedVersionResolver
}

type ErrorHandler = httpx.ErrorHandler

// TransitionAuthorizer authorizes each transition attempt using current
// stored model state. Page/PageFunc initialization remains governed by Init
// and is not gated here; a missing model created during Handle is gated.
type TransitionAuthorizer[K comparable, Model any, Msg mvu.Message] func(
	context.Context, K, Model, Msg,
) error

type RuntimeConfig[K comparable, Model any, Msg mvu.Message] struct {
	App   mvu.Application[Model, Msg]
	Store store.Store[K, Model]
	Keys  KeyResolver[K]
	View  View[Model]
	// Theme is the active visual theme. Nil → theme.DefaultTheme.
	Theme theme.Theme
	// Themes resolves color mode. Nil → theme.ModeFromContext.
	Themes ThemeResolver
	// Commands runs effects. Nil → SyncCommands.
	Commands CommandRunner[Msg]
	// OnError handles errors before the response starts. Nil → 500 text.
	OnError ErrorHandler
	// AuthorizeTransition runs before every Update/CAS attempt and before
	// Handle creates a missing model for that intent. Page initialization
	// remains ungated. HX headers are never authorization evidence.
	AuthorizeTransition TransitionAuthorizer[K, Model, Msg]
	// ObserveOutcome receives one commit-aware completion per routed request.
	ObserveOutcome OutcomeObserver
	// Logger records errors after the response started. Nil → slog.Default.
	Logger     *slog.Logger
	MaxRetries int
}

type Runtime[K comparable, Model any, Msg mvu.Message] struct {
	app        mvu.Application[Model, Msg]
	store      store.Store[K, Model]
	keys       KeyResolver[K]
	view       View[Model]
	theme      theme.Theme
	themes     ThemeResolver
	commands   CommandRunner[Msg]
	onError    ErrorHandler
	authorize  TransitionAuthorizer[K, Model, Msg]
	observe    OutcomeObserver
	logger     *slog.Logger
	maxRetries int
}

// NewRuntime validates config and applies defaults.
func NewRuntime[K comparable, Model any, Msg mvu.Message](
	config RuntimeConfig[K, Model, Msg],
) (*Runtime[K, Model, Msg], error) {
	if config.App == nil || config.Store == nil || config.Keys == nil || config.View == nil {
		return nil, ErrInvalidRuntime
	}
	runtime := &Runtime[K, Model, Msg]{
		app: config.App, store: config.Store, keys: config.Keys, view: config.View,
		theme: config.Theme, themes: config.Themes, commands: config.Commands,
		onError: config.OnError, authorize: config.AuthorizeTransition,
		observe: config.ObserveOutcome, logger: config.Logger, maxRetries: config.MaxRetries,
	}
	if runtime.theme == nil {
		runtime.theme = theme.DefaultTheme{}
	}
	if runtime.commands == nil {
		runtime.commands = SyncCommands[Msg]{}
	}
	if runtime.onError == nil {
		runtime.onError = InternalServerError
	}
	if runtime.logger == nil {
		runtime.logger = slog.Default()
	}
	if runtime.maxRetries <= 0 {
		runtime.maxRetries = 3
	}
	return runtime, nil
}

// MustRuntime panics on invalid config. For main and tests.
func MustRuntime[K comparable, Model any, Msg mvu.Message](
	config RuntimeConfig[K, Model, Msg],
) *Runtime[K, Model, Msg] {
	runtime, err := NewRuntime(config)
	if err != nil {
		panic(err)
	}
	return runtime
}

// InternalServerError writes a generic 500 without leaking err.
func InternalServerError(w http.ResponseWriter, r *http.Request, err error) {
	httpx.RespondError(w, r, err)
}

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
		key, err := runtime.keys.Resolve(r)
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
			version, err := route.ExpectedVersion.ResolveVersion(r)
			if err != nil {
				fail(err)
				return
			}
			expected = &version
		}
		trace := timing.FromContext(ctx)
		if route.RequireModel {
			if err := ctx.Err(); err != nil {
				fail(err)
				return
			}
			done := trace.Start(timing.PhaseLoad)
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
		done := trace.Start(timing.PhaseDecode)
		message, err := route.Decode.Decode(r)
		done()
		if err != nil {
			fail(err)
			return
		}
		if err := ctx.Err(); err != nil {
			fail(err)
			return
		}
		done = trace.Start(timing.PhaseDispatch)
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
		done = trace.Start(timing.PhaseLoad)
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
		key, err := runtime.keys.Resolve(r)
		if err != nil {
			fail(err)
			return
		}
		if err := r.Context().Err(); err != nil {
			fail(err)
			return
		}
		trace := timing.FromContext(r.Context())
		done := trace.Start(timing.PhaseLoad)
		snapshot, commands, created, err := runtime.loadOrCreate(r.Context(), key)
		done()
		if err != nil {
			fail(err)
			return
		}
		committed.Store(created)
		if len(commands) > 0 {
			done = trace.Start(timing.PhaseDispatch)
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
			done = trace.Start(timing.PhaseLoad)
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
	if runtime.observe == nil {
		return
	}
	state := OutcomeRejected
	switch {
	case panicValue != nil && tracked.Started:
		state = OutcomeResponseAborted
	case isEffectError(err):
		state = OutcomeEffectFailed
	case isResponseAbort(err):
		state = OutcomeResponseAborted
	case committed:
		state = OutcomeCommitted
	case err == nil && panicValue == nil:
		state = OutcomeCompleted
	}
	if panicValue != nil && err == nil {
		err = fmt.Errorf("request panic: %v", panicValue)
	}
	runtime.observe(r, RequestOutcome{State: state, Committed: committed, Err: err})
}

// dispatcher returns a Dispatch that transitions the model for key and
// hands follow-up commands to the CommandRunner. Commands never run while
// the store holds a lock: CAS completes before Run.
func (runtime *Runtime[K, Model, Msg]) dispatcher(
	key K, expected *store.Version, onCommit func(),
) mvu.Dispatch[Msg] {
	var followup mvu.Dispatch[Msg]
	followup = func(ctx context.Context, message Msg) error {
		return runtime.dispatchMessage(ctx, key, message, nil, onCommit, followup)
	}
	return func(ctx context.Context, message Msg) error {
		return runtime.dispatchMessage(ctx, key, message, expected, onCommit, followup)
	}
}

func (runtime *Runtime[K, Model, Msg]) dispatchMessage(
	ctx context.Context, key K, message Msg, expected *store.Version,
	onCommit func(), followup mvu.Dispatch[Msg],
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	commands, err := runtime.transition(ctx, key, message, expected, onCommit)
	if err != nil {
		return err
	}
	if len(commands) == 0 {
		return nil
	}
	if err := runtime.commands.Run(ctx, commands, followup); err != nil {
		if onCommit != nil {
			return &EffectError{Cause: err}
		}
		return err
	}
	return nil
}

func (runtime *Runtime[K, Model, Msg]) loadOrCreate(
	ctx context.Context, key K,
) (store.Snapshot[Model], []mvu.Command[Msg], bool, error) {
	for range runtime.maxRetries {
		if err := ctx.Err(); err != nil {
			return store.Snapshot[Model]{}, nil, false, err
		}
		snapshot, err := runtime.store.Load(ctx, key)
		if err == nil {
			return snapshot, nil, false, nil
		}
		if !errors.Is(err, store.ErrNotFound) {
			return store.Snapshot[Model]{}, nil, false, err
		}
		model, commands, err := runtime.app.Init(ctx)
		if err != nil {
			return store.Snapshot[Model]{}, nil, false, err
		}
		if err := ctx.Err(); err != nil {
			return store.Snapshot[Model]{}, nil, false, err
		}
		snapshot, err = runtime.store.Create(ctx, key, model)
		if err == nil {
			return snapshot, commands, true, nil
		}
		if !errors.Is(err, store.ErrAlreadyExists) {
			return store.Snapshot[Model]{}, nil, false, err
		}
	}
	return store.Snapshot[Model]{}, nil, false, store.ErrConflict
}

func (runtime *Runtime[K, Model, Msg]) transition(
	ctx context.Context, key K, message Msg, expected *store.Version, onCommit func(),
) ([]mvu.Command[Msg], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot, err := runtime.store.Load(ctx, key)
	var initCommands []mvu.Command[Msg]
	if errors.Is(err, store.ErrNotFound) {
		model, commands, initErr := runtime.app.Init(ctx)
		if initErr != nil {
			return nil, initErr
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if runtime.authorize != nil {
			if authErr := runtime.authorize(ctx, key, model, message); authErr != nil {
				return nil, authErr
			}
		}
		snapshot, err = runtime.store.Create(ctx, key, model)
		if errors.Is(err, store.ErrAlreadyExists) {
			snapshot, err = runtime.store.Load(ctx, key)
		} else if err == nil {
			initCommands = commands
			if onCommit != nil {
				onCommit()
			}
		}
	}
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < runtime.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if expected != nil && snapshot.Version != *expected {
			return nil, staleVersionError()
		}
		if runtime.authorize != nil {
			if err := runtime.authorize(ctx, key, snapshot.Model, message); err != nil {
				return nil, err
			}
		}
		model, commands, err := runtime.app.Update(ctx, snapshot.Model, message)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		_, err = runtime.store.CompareAndSwap(ctx, key, snapshot.Version, model)
		if err == nil {
			if onCommit != nil {
				onCommit()
			}
			return append(initCommands, commands...), nil
		}
		if !errors.Is(err, store.ErrConflict) {
			return nil, err
		}
		if expected != nil {
			return nil, staleVersionError()
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if snapshot, err = runtime.store.Load(ctx, key); err != nil {
			return nil, err
		}
	}
	return nil, store.ErrConflict
}

func (runtime *Runtime[K, Model, Msg]) writeResult(
	w http.ResponseWriter, r *http.Request, model Model, result Result,
) error {
	tracked, ok := w.(*httpx.Writer)
	if !ok {
		tracked = &httpx.Writer{ResponseWriter: w}
		w = tracked
	}
	request := NewRequestContext(r, runtime.theme, runtime.themes)
	mode := result.Mode
	cacheControl := result.CacheControl
	if cacheControl == "" {
		cacheControl = "no-store"
	}
	w.Header().Set("Cache-Control", cacheControl)
	if mode == RenderAuto {
		httpx.Vary(w.Header(), "HX-Request", "HX-Boosted", "HX-History-Restore-Request")
		switch {
		case request.IsHTMX && !request.HTMX.IsBoosted && !request.HTMX.IsHistory:
			mode = RenderFragment
		case result.Location != "":
			http.Redirect(w, r, result.Location, result.StatusOr(http.StatusSeeOther))
			return nil
		default:
			mode = RenderPage
		}
	} else if result.Location != "" {
		http.Redirect(w, r, result.Location, result.StatusOr(http.StatusSeeOther))
		return nil
	}
	if err := r.Context().Err(); err != nil {
		runtime.fail(w, r, err)
		return err
	}
	if result.PushURL != "" {
		htmx.PushURL(w, result.PushURL)
	}
	if result.ReplaceURL != "" {
		htmx.ReplaceURL(w, result.ReplaceURL)
	}
	trace := timing.FromContext(r.Context())
	var err error
	switch mode {
	case RenderNothing:
		NoContent(w)
		return nil
	case RenderPage:
		var page html.Page
		done := trace.Start(timing.PhaseView)
		page, err = runtime.view.Page(r.Context(), model, request)
		done()
		if err == nil {
			err = r.Context().Err()
		}
		if err == nil {
			done = trace.Start(timing.PhaseRender)
			err = WritePage(r.Context(), tracked, result.StatusOr(http.StatusOK), page)
			done()
		}
	case RenderFragment:
		var fragment html.Fragment
		done := trace.Start(timing.PhaseView)
		fragment, err = runtime.view.Fragment(r.Context(), result.Fragment, model, request)
		done()
		if err == nil {
			err = r.Context().Err()
		}
		if err == nil {
			done = trace.Start(timing.PhaseRender)
			err = WriteFragment(r.Context(), tracked, result.StatusOr(http.StatusOK), fragment)
			done()
		}
	default:
		err = fmt.Errorf("%w: %d", ErrUnknownRender, mode)
	}
	if err == nil {
		return nil
	}
	if tracked.Started {
		if errors.Is(err, http.ErrAbortHandler) {
			panic(http.ErrAbortHandler)
		}
		runtime.logger.ErrorContext(r.Context(), "render failed after response started",
			"path", r.URL.Path, "error", err)
		return &responseAbortError{cause: err}
	}
	runtime.fail(w, r, err)
	return err
}

func (runtime *Runtime[K, Model, Msg]) fail(w http.ResponseWriter, r *http.Request, err error) {
	err = normalizeError(err)
	runtime.onError(w, r, err)
}

type responseAbortError struct{ cause error }

func (e *responseAbortError) Error() string { return "response aborted: " + e.cause.Error() }
func (e *responseAbortError) Unwrap() error { return e.cause }

func isEffectError(err error) bool {
	var effect *EffectError
	return errors.As(err, &effect)
}

func isResponseAbort(err error) bool {
	var aborted *responseAbortError
	return errors.As(err, &aborted)
}

func staleVersionError() error {
	return &httpx.Error{
		Status: http.StatusConflict, Public: http.StatusText(http.StatusConflict),
		Cause: store.ErrConflict,
	}
}

func normalizeError(err error) error {
	if isEffectError(err) {
		return &httpx.Error{
			Status: http.StatusInternalServerError, Public: http.StatusText(http.StatusInternalServerError),
			Cause: err,
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return &httpx.Error{
			Status: http.StatusRequestEntityTooLarge, Public: http.StatusText(http.StatusRequestEntityTooLarge),
			Cause: err,
		}
	}
	return err
}

// WritePage renders into a buffer-free stream. Headers are sent before the
// body, so a mid-render failure yields a truncated 200; callers log it.
func WritePage(ctx context.Context, w http.ResponseWriter, status int, page html.Page) error {
	if page == nil {
		return errors.New("nil HTML page")
	}
	return write(ctx, w, status, page)
}

func WriteFragment(ctx context.Context, w http.ResponseWriter, status int, fragment html.Fragment) error {
	if fragment == nil {
		return errors.New("nil HTML fragment")
	}
	return write(ctx, w, status, fragment)
}

func write(ctx context.Context, w http.ResponseWriter, status int, component html.Component) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	return html.Render(ctx, w, component)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
