package web

import (
	"context"
	"net/http"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/store"
	"github.com/dmundt/go-mvu/theme"
)

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
