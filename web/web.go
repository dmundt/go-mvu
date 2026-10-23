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

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/store"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/timing"
)

var (
	ErrUnknownFragment = errors.New("unknown HTML fragment")
	ErrInvalidRuntime  = errors.New("incomplete web runtime configuration")
	ErrUnknownRender   = errors.New("unknown render mode")
)

// RequestContext carries per-request presentation state into views.
type RequestContext struct {
	Theme      theme.Theme
	Mode       theme.Mode
	IsHTMX     bool
	HTMX       htmx.Request
	CurrentURL string
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
		HTMX: request, CurrentURL: current,
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

// AsyncCommands runs each command in its own goroutine, detached from
// request cancellation. Errors go to OnError (default: slog).
// Response renders before commands finish.
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
}

type ErrorHandler func(http.ResponseWriter, *http.Request, error)

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
		onError: config.OnError, logger: config.Logger, maxRetries: config.MaxRetries,
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
func InternalServerError(w http.ResponseWriter, _ *http.Request, _ error) {
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// Handle decodes a message, applies Update, runs commands, renders Result.
func (runtime *Runtime[K, Model, Msg]) Handle(route Route[Msg]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route.Pattern != "" && r.URL.Path != route.Pattern {
			http.NotFound(w, r)
			return
		}
		if route.Method != "" && r.Method != route.Method {
			w.Header().Set("Allow", route.Method)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		if route.Decode == nil {
			runtime.fail(w, r, ErrInvalidRuntime)
			return
		}
		key, err := runtime.keys.Resolve(r)
		if err != nil {
			runtime.fail(w, r, err)
			return
		}
		ctx := r.Context()
		trace := timing.FromContext(ctx)
		if route.RequireModel {
			done := trace.Start(timing.PhaseLoad)
			_, err := runtime.store.Load(ctx, key)
			done()
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					http.NotFound(w, r)
				} else {
					runtime.fail(w, r, err)
				}
				return
			}
		}
		done := trace.Start(timing.PhaseDecode)
		message, err := route.Decode.Decode(r)
		done()
		if err != nil {
			runtime.fail(w, r, err)
			return
		}
		done = trace.Start(timing.PhaseDispatch)
		err = runtime.dispatcher(key)(ctx, message)
		done()
		if err != nil {
			runtime.fail(w, r, err)
			return
		}
		done = trace.Start(timing.PhaseLoad)
		snapshot, err := runtime.store.Load(ctx, key)
		done()
		if err != nil {
			runtime.fail(w, r, err)
			return
		}
		runtime.writeResult(w, r, snapshot.Model, route.Result)
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
		if result == nil {
			runtime.fail(w, r, ErrInvalidRuntime)
			return
		}
		key, err := runtime.keys.Resolve(r)
		if err != nil {
			runtime.fail(w, r, err)
			return
		}
		trace := timing.FromContext(r.Context())
		done := trace.Start(timing.PhaseLoad)
		snapshot, commands, err := runtime.loadOrCreate(r.Context(), key)
		done()
		if err != nil {
			runtime.fail(w, r, err)
			return
		}
		if len(commands) > 0 {
			done = trace.Start(timing.PhaseDispatch)
			err = runtime.commands.Run(r.Context(), commands, runtime.dispatcher(key))
			done()
			if err != nil {
				runtime.fail(w, r, err)
				return
			}
			done = trace.Start(timing.PhaseLoad)
			snapshot, err = runtime.store.Load(r.Context(), key)
			done()
			if err != nil {
				runtime.fail(w, r, err)
				return
			}
		}
		runtime.writeResult(w, r, snapshot.Model, result(r))
	})
}

// dispatcher returns a Dispatch that transitions the model for key and
// hands follow-up commands to the CommandRunner. Commands never run while
// the store holds a lock: CAS completes before Run.
func (runtime *Runtime[K, Model, Msg]) dispatcher(key K) mvu.Dispatch[Msg] {
	var dispatch mvu.Dispatch[Msg]
	dispatch = func(ctx context.Context, message Msg) error {
		commands, err := runtime.transition(ctx, key, message)
		if err != nil {
			return err
		}
		if len(commands) == 0 {
			return nil
		}
		return runtime.commands.Run(ctx, commands, dispatch)
	}
	return dispatch
}

func (runtime *Runtime[K, Model, Msg]) loadOrCreate(
	ctx context.Context, key K,
) (store.Snapshot[Model], []mvu.Command[Msg], error) {
	for range runtime.maxRetries {
		snapshot, err := runtime.store.Load(ctx, key)
		if err == nil {
			return snapshot, nil, nil
		}
		if !errors.Is(err, store.ErrNotFound) {
			return store.Snapshot[Model]{}, nil, err
		}
		model, commands, err := runtime.app.Init(ctx)
		if err != nil {
			return store.Snapshot[Model]{}, nil, err
		}
		snapshot, err = runtime.store.Create(ctx, key, model)
		if err == nil {
			return snapshot, commands, nil
		}
		if !errors.Is(err, store.ErrAlreadyExists) {
			return store.Snapshot[Model]{}, nil, err
		}
	}
	return store.Snapshot[Model]{}, nil, store.ErrConflict
}

func (runtime *Runtime[K, Model, Msg]) transition(
	ctx context.Context, key K, message Msg,
) ([]mvu.Command[Msg], error) {
	snapshot, initCommands, err := runtime.loadOrCreate(ctx, key)
	if err != nil {
		return nil, err
	}
	for range runtime.maxRetries {
		model, commands, err := runtime.app.Update(ctx, snapshot.Model, message)
		if err != nil {
			return nil, err
		}
		_, err = runtime.store.CompareAndSwap(ctx, key, snapshot.Version, model)
		if err == nil {
			return append(initCommands, commands...), nil
		}
		if !errors.Is(err, store.ErrConflict) {
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
) {
	request := NewRequestContext(r, runtime.theme, runtime.themes)
	mode := result.Mode
	if mode == RenderAuto {
		switch {
		case request.IsHTMX && !request.HTMX.IsBoosted && !request.HTMX.IsHistory:
			mode = RenderFragment
		case result.Location != "":
			http.Redirect(w, r, result.Location, result.StatusOr(http.StatusSeeOther))
			return
		default:
			mode = RenderPage
		}
	} else if result.Location != "" {
		http.Redirect(w, r, result.Location, result.StatusOr(http.StatusSeeOther))
		return
	}
	if result.PushURL != "" {
		htmx.PushURL(w, result.PushURL)
	}
	if result.ReplaceURL != "" {
		htmx.ReplaceURL(w, result.ReplaceURL)
	}
	tracked := &trackingWriter{ResponseWriter: w}
	trace := timing.FromContext(r.Context())
	var err error
	switch mode {
	case RenderNothing:
		NoContent(w)
		return
	case RenderPage:
		var page html.Page
		done := trace.Start(timing.PhaseView)
		page, err = runtime.view.Page(r.Context(), model, request)
		done()
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
			done = trace.Start(timing.PhaseRender)
			err = WriteFragment(r.Context(), tracked, result.StatusOr(http.StatusOK), fragment)
			done()
		}
	default:
		err = fmt.Errorf("%w: %d", ErrUnknownRender, mode)
	}
	if err == nil {
		return
	}
	if tracked.started {
		runtime.logger.ErrorContext(r.Context(), "render failed after response started",
			"path", r.URL.Path, "error", err)
		return
	}
	runtime.onError(w, r, err)
}

func (runtime *Runtime[K, Model, Msg]) fail(w http.ResponseWriter, r *http.Request, err error) {
	runtime.onError(w, r, err)
}

// trackingWriter records whether headers or body were sent.
type trackingWriter struct {
	http.ResponseWriter
	started bool
}

func (w *trackingWriter) WriteHeader(status int) {
	w.started = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *trackingWriter) Write(data []byte) (int, error) {
	w.started = true
	return w.ResponseWriter.Write(data)
}

func (w *trackingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

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
