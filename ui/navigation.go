package ui

import (
	"context"
	"errors"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/theme"
)

var ErrInvalidLink = errors.New("ui: action link requires a GET action")

// ActionLink uses one GET action for both native and enhanced navigation.
type ActionLink struct {
	Action  htmx.Action
	Label   string
	Variant components.ButtonVariant
	Attrs   html.Attrs
}

func (link ActionLink) RenderHTML(ctx context.Context, out *html.Context) error {
	if link.Action.Method != htmx.Get {
		return ErrInvalidLink
	}
	if err := link.Action.Validate(); err != nil {
		return err
	}
	return (components.Link{
		URL: link.Action.URL, Action: link.Action, Label: link.Label,
		Variant: link.Variant, Attrs: link.Attrs,
	}).RenderHTML(ctx, out)
}

// ThemeSwitcher includes the session CSRF field. The application must install
// CSRF middleware and the theme preference handler separately.
type ThemeSwitcher struct {
	Mode   theme.Mode
	Return string
}

func (switcher ThemeSwitcher) RenderHTML(ctx context.Context, out *html.Context) error {
	// Resolve the token before emitting any markup.
	if _, ok := middleware.CSRFToken(ctx); !ok {
		return middleware.ErrCSRFTokenUnset
	}
	return (components.ThemeSwitcher{
		Mode: switcher.Mode, Return: switcher.Return, CSRFField: middleware.CSRFInput{},
	}).RenderHTML(ctx, out)
}
