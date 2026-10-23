package ui

import (
	"context"
	"errors"
	"strings"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/middleware"
)

var ErrInvalidForm = errors.New("ui: form requires a same-origin GET or POST action")

// Form derives both plain HTML and HTMX submission from Action. POST always
// includes a CSRF field and requires middleware.CSRF.Wrap. GET never sends it.
// PUT, PATCH and DELETE have no native form fallback and are rejected.
type Form struct {
	Action htmx.Action
	Body   html.Component
	Attrs  html.Attrs
}

func (form Form) RenderHTML(ctx context.Context, out *html.Context) error {
	method := components.FormPost
	switch form.Action.Method {
	case htmx.Get:
		method = components.FormGet
	case htmx.Post:
	default:
		return ErrInvalidForm
	}
	if !strings.HasPrefix(form.Action.URL, "/") ||
		strings.HasPrefix(form.Action.URL, "//") ||
		strings.ContainsAny(form.Action.URL, "\\\r\n\t") {
		return ErrInvalidForm
	}
	if err := form.Action.Validate(); err != nil {
		return err
	}
	out.Component(ctx, components.Form{
		Method:    method,
		URL:       form.Action.URL,
		Action:    form.Action,
		CSRFField: middleware.CSRFInput{},
		Children:  []html.Component{form.Body},
		Attrs:     form.Attrs,
	})
	return out.Err()
}

// ActionButton is a standalone submit button with a native form fallback.
// Attrs apply to the form; use Form with Button for custom button attributes.
type ActionButton struct {
	Action    htmx.Action
	Label     string
	AriaLabel string
	Variant   components.ButtonVariant
	Disabled  bool
	Attrs     html.Attrs
}

func (button ActionButton) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, Form{
		Action: button.Action,
		Attrs:  button.Attrs,
		Body: components.Button{
			Type:      components.ButtonSubmit,
			Label:     button.Label,
			AriaLabel: button.AriaLabel,
			Variant:   button.Variant,
			Disabled:  button.Disabled,
		},
	})
	return out.Err()
}
