package web

import (
	"context"
	"net/http"

	"github.com/dmundt/go-mvu/core"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
)

// RequestContext carries per-request presentation state into views.
type RequestContext struct {
	Theme      theme.Theme
	Mode       theme.Mode
	IsHTMX     bool
	HTMX       HTMXRequest
	CurrentURL string
	// Locale is explicitly resolved by HTTP middleware and passed to views.
	Locale string
}

// NewRequestContext builds view state. Nil theme → theme.DefaultTheme.
// Mode comes from theme.WithMode or theme.Preference.Wrap (System default).
func NewRequestContext(r *http.Request, active theme.Theme) RequestContext {
	if active == nil {
		active = theme.DefaultTheme{}
	}
	mode := theme.ModeFromContext(r.Context())
	request := HTMXFromRequest(r)
	current := r.URL.RequestURI()
	if request.IsHTMX && request.CurrentURL != "" {
		current = request.CurrentURL
	}
	return RequestContext{
		Theme: active, Mode: mode, IsHTMX: request.IsHTMX,
		HTMX: request, CurrentURL: current, Locale: Locale(r.Context()),
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

// Decoder parses a request into an application message.
type Decoder[Msg core.Message] func(*http.Request) (Msg, error)

// StaticMessage returns a decoder for a request-independent message.
// Messages containing mutable data must not be mutated by Update.
func StaticMessage[Msg core.Message](message Msg) Decoder[Msg] {
	return func(*http.Request) (Msg, error) { return message, nil }
}

type KeyResolver[K comparable] func(*http.Request) (K, error)

type ExpectedVersion func(*http.Request) (core.Version, error)

type Route[Msg core.Message] struct {
	Method  string
	Pattern string
	Decode  Decoder[Msg]
	Result  Result
	// RequireModel disables Init for this route: a missing model yields 404
	// instead of being created.
	RequireModel bool
	// ExpectedVersion is trusted application parsing of an intent version.
	// A stale intent is rejected with 409, never retried against newer state.
	ExpectedVersion ExpectedVersion
}
