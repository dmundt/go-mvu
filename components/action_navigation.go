package components

import (
	"context"
	"errors"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

var ErrInvalidLink = errors.New("components: action link requires a GET action")

// ActionLink uses one GET action for both native and enhanced navigation.
type ActionLink struct {
	Action  htmx.Action
	Label   string
	Variant ButtonVariant
	Attrs   html.Attrs
}

func (link ActionLink) RenderHTML(ctx context.Context, out *html.Context) error {
	if link.Action.Method != htmx.Get {
		return ErrInvalidLink
	}
	if err := link.Action.Validate(); err != nil {
		return err
	}
	return (Link{
		URL: link.Action.URL, Action: link.Action, Label: link.Label,
		Variant: link.Variant, Attrs: link.Attrs,
	}).RenderHTML(ctx, out)
}

// SecureThemeSwitcher includes the session CSRF field. The application must install
// CSRF middleware and the theme preference handler separately.
type SecureThemeSwitcher struct {
	Mode   theme.Mode
	Return string
}

func (switcher SecureThemeSwitcher) RenderHTML(ctx context.Context, out *html.Context) error {
	// Resolve the token before emitting any markup.
	if _, ok := web.CSRFToken(ctx); !ok {
		return web.ErrCSRFTokenUnset
	}
	return (ThemeSwitcher{
		Mode: switcher.Mode, Return: switcher.Return, CSRFField: CSRFInput{},
	}).RenderHTML(ctx, out)
}
