// Package web binds MVU applications to net/http.
//
// Request flow: resolve key → decode message → load or create model →
// Update with compare-and-swap retry → run commands outside the store →
// render page or fragment with a RequestContext.
package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/store"
	"github.com/dmundt/go-mvu/theme"
)

var (
	ErrUnknownFragment            = errors.New("unknown HTML fragment")
	ErrInvalidRuntime             = errors.New("incomplete web runtime configuration")
	ErrUnknownRender              = errors.New("unknown render mode")
	ErrCommandOverload            = errors.New("async command queue full")
	ErrCommandClosed              = errors.New("async command runner closed")
	ErrInvalidCommandRunnerConfig = errors.New("invalid async command runner configuration")
)

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
