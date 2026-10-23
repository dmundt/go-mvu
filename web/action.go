package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/mvu"
)

var ErrInvalidAction = errors.New("invalid web action")

// Action keeps one endpoint's HTTP route and browser behavior together.
// An empty target and swap use the fragment name as "#<fragment>" and
// outerHTML. Non-GET RenderAuto actions redirect plain submissions to Path
// unless Result.Location is set.
type Action[Msg mvu.Message] struct {
	Method          htmx.Method
	Path            string
	Decode          Decoder[Msg]
	Result          Result
	Target          string
	Swap            htmx.Swap
	RequireModel    bool
	ExpectedVersion ExpectedVersion
}

// PostMessage defines a POST action for a static message. Result remains
// explicit so fragment and plain-form redirect policy stay visible.
func PostMessage[Msg mvu.Message](path string, message Msg, result Result) Action[Msg] {
	return Action[Msg]{
		Method: htmx.Post,
		Path:   path,
		Decode: StaticMessage(message),
		Result: result,
	}
}

// RegisterAll mounts actions in order, stopping on the first error. Previously
// registered actions remain mounted; use only during application initialization.
func (runtime *Runtime[K, Model, Msg]) RegisterAll(
	mux *http.ServeMux, actions ...Action[Msg],
) error {
	if runtime == nil || mux == nil {
		return ErrInvalidAction
	}
	for _, action := range actions {
		if err := runtime.Register(mux, action); err != nil {
			return err
		}
	}
	return nil
}

// HTMX returns the browser action described by this route.
func (action Action[Msg]) HTMX() htmx.Action {
	target, swap := action.Target, action.Swap
	if target == "" && action.Result.Fragment != "" {
		target = "#" + action.Result.Fragment
	}
	if swap == "" && action.Result.Fragment != "" {
		swap = htmx.OuterHTML
	}
	return htmx.Action{
		Method: action.Method,
		URL:    action.Path,
		Target: target,
		Swap:   swap,
	}
}

// Register validates and mounts action on mux using its exact path and method.
func (runtime *Runtime[K, Model, Msg]) Register(
	mux *http.ServeMux,
	action Action[Msg],
) error {
	if runtime == nil || mux == nil {
		return ErrInvalidAction
	}
	method := action.httpMethod()
	if method == "" || action.Decode == nil ||
		!strings.HasPrefix(action.Path, "/") ||
		strings.HasPrefix(action.Path, "//") ||
		strings.ContainsAny(action.Path, "?#{}") {
		return ErrInvalidAction
	}
	if err := action.HTMX().Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidAction, err)
	}

	result := action.Result
	if result.Mode == RenderAuto && result.Location == "" && action.Method != htmx.Get {
		result.Location = action.Path
	}
	mux.Handle(method+" "+action.Path, runtime.Handle(Route[Msg]{
		Method:          method,
		Pattern:         action.Path,
		Decode:          action.Decode,
		Result:          result,
		RequireModel:    action.RequireModel,
		ExpectedVersion: action.ExpectedVersion,
	}))
	return nil
}

func (action Action[Msg]) httpMethod() string {
	switch action.Method {
	case htmx.Get:
		return http.MethodGet
	case htmx.Post:
		return http.MethodPost
	case htmx.Put:
		return http.MethodPut
	case htmx.Patch:
		return http.MethodPatch
	case htmx.Delete:
		return http.MethodDelete
	default:
		return ""
	}
}
