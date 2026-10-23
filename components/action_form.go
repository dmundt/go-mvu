package components

import (
	"context"
	"errors"
	"strings"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

var ErrInvalidForm = errors.New("components: form requires a same-origin GET or POST action")

// ActionForm derives both plain HTML and HTMX submission from Action. POST always
// includes a CSRF field and requires web.CSRF.Wrap. GET never sends it.
// PUT, PATCH and DELETE have no native form fallback and are rejected.
type ActionForm struct {
	Action web.BrowserAction
	Body   html.Component
	Attrs  html.Attrs
}

func (form ActionForm) RenderHTML(ctx context.Context, out *html.Context) error {
	method := FormPost
	switch form.Action.Method {
	case web.Get:
		method = FormGet
	case web.Post:
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
	out.Component(ctx, Form{
		Method:    method,
		URL:       form.Action.URL,
		Action:    form.Action,
		CSRFField: CSRFInput{},
		Children:  []html.Component{form.Body},
		Attrs:     form.Attrs,
	})
	return out.Err()
}

// ActionButton is a standalone submit button with a native form fallback.
// Attrs apply to the form; use ActionForm with Button for custom button attributes.
type ActionButton struct {
	Action    web.BrowserAction
	Label     string
	AriaLabel string
	Variant   ButtonVariant
	Disabled  bool
	Attrs     html.Attrs
}

func (button ActionButton) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, ActionForm{
		Action: button.Action,
		Attrs:  button.Attrs,
		Body: Button{
			Type:      ButtonSubmit,
			Label:     button.Label,
			AriaLabel: button.AriaLabel,
			Variant:   button.Variant,
			Disabled:  button.Disabled,
		},
	})
	return out.Err()
}
