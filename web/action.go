package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dmundt/go-mvu/core"
)

var ErrInvalidAction = errors.New("invalid web action")

// Action keeps one endpoint's HTTP route and browser behavior together.
// An empty target and swap use the fragment name as "#<fragment>" and
// outerHTML. Non-GET RenderAuto actions redirect plain submissions to Path
// unless Result.Location is set.
type Action[Msg core.Message] struct {
	Method          Method
	Path            string
	Decode          Decoder[Msg]
	Result          Result
	Target          string
	Swap            Swap
	RequireModel    bool
	ExpectedVersion ExpectedVersion
}

// PostMessage defines a POST action for a static message. Result remains
// explicit so fragment and plain-form redirect policy stay visible.
func PostMessage[Msg core.Message](path string, message Msg, result Result) Action[Msg] {
	return Action[Msg]{
		Method: Post,
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
func (action Action[Msg]) HTMX() BrowserAction {
	target, swap := action.Target, action.Swap
	if target == "" && action.Result.Fragment != "" {
		target = "#" + action.Result.Fragment
	}
	if swap == "" && action.Result.Fragment != "" {
		swap = OuterHTML
	}
	return BrowserAction{
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
	if result.Mode == RenderAuto && result.Location == "" && action.Method != Get {
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
	case Get:
		return http.MethodGet
	case Post:
		return http.MethodPost
	case Put:
		return http.MethodPut
	case Patch:
		return http.MethodPatch
	case Delete:
		return http.MethodDelete
	default:
		return ""
	}
}
