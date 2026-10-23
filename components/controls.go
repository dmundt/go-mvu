package components

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

type ButtonType string
type ButtonVariant string

const (
	ButtonSubmit ButtonType = "submit"
	ButtonReset  ButtonType = "reset"
	ButtonButton ButtonType = "button"

	ButtonPrimary   ButtonVariant = "primary"
	ButtonSecondary ButtonVariant = "secondary"
	ButtonDanger    ButtonVariant = "danger"
	ButtonGhost     ButtonVariant = "ghost"
)

type Button struct {
	ID       string
	Type     ButtonType
	Label    string
	Variant  ButtonVariant
	Disabled bool
	Action   web.BrowserAction
	// AriaLabel names icon-only or symbol buttons such as "+" or "×".
	AriaLabel string
	// Attrs adds passthrough attributes. With Action set, hx-* names are
	// owned by Action and skipped.
	Attrs html.Attrs
}

func validButtonVariant(variant ButtonVariant) bool {
	switch variant {
	case "", ButtonPrimary, ButtonSecondary, ButtonDanger, ButtonGhost:
		return true
	default:
		return false
	}
}

func buttonClass(out *html.Context, variant ButtonVariant, attrs html.Attrs) {
	out.Static(`class="button`)
	if variant != "" {
		out.Static(" button--")
		out.Attribute(string(variant))
	}
	out.ExtraClass(attrs)
	out.Static(`"`)
}

// actionOwned returns "hx-*" when action renders htmx attributes.
func actionOwned(action web.BrowserAction) string {
	if action.Method == 0 {
		return ""
	}
	return "hx-*"
}

// ThemeSwitcher posts mode=system|light|dark to Action (default "/theme").
// Works without JavaScript (303 redirect) and with HTMX (HX-Refresh).
// Pair with theme.Preference.Handler.
type ThemeSwitcher struct {
	Mode   theme.Mode
	Action string
	// Target is an optional HTMX target selector (for example "#app")
	// swapped with outerHTML; empty means no swap (the handler refreshes).
	Target string
	// Return is the same-origin path to come back to after a non-HTMX post.
	Return    string
	CSRFField html.Component
	Attrs     html.Attrs
}

func (switcher ThemeSwitcher) RenderHTML(ctx context.Context, out *html.Context) error {
	path := switcher.Action
	if path == "" {
		path = "/theme"
	}
	if err := validateURL(path); err != nil {
		return fmt.Errorf("invalid theme switcher path: %w", err)
	}
	out.Static(`<form class="theme-switcher`)
	out.ExtraClass(switcher.Attrs)
	out.Static(`" method="post" action="`)
	out.Attribute(path)
	out.Static(`" aria-label="Color theme"`)
	out.PassAttrs(switcher.Attrs, "method", "action", "aria-label", "hx-*")
	action := web.BrowserAction{Method: web.Post, URL: path, Swap: web.NoSwap}
	if switcher.Target != "" {
		action.Target = switcher.Target
		action.Swap = web.OuterHTML
	}
	if err := action.RenderAttributes(out); err != nil {
		return err
	}
	out.Static(">")
	out.Component(ctx, switcher.CSRFField)
	if switcher.Return != "" {
		out.Static(`<input type="hidden" name="` + theme.ReturnField + `" value="`)
		out.Attribute(theme.SafeReturnPath(switcher.Return))
		out.Static(`">`)
	}
	for _, mode := range theme.Modes {
		out.Static(`<button type="submit" name="` + theme.ModeField + `" value="`)
		out.Static(mode.String())
		out.Static(`" class="button button--`)
		if mode == switcher.Mode {
			out.Static(`primary" aria-pressed="true">`)
		} else {
			out.Static(`secondary" aria-pressed="false">`)
		}
		out.Text(mode.Label())
		out.Static("</button>")
	}
	out.Static("</form>")
	return out.Err()
}

func (button Button) RenderHTML(_ context.Context, out *html.Context) error {
	buttonType := button.Type
	if buttonType == "" {
		buttonType = ButtonButton
	}
	if buttonType != ButtonButton && buttonType != ButtonSubmit && buttonType != ButtonReset {
		return fmt.Errorf("invalid button type %q", buttonType)
	}
	if !validButtonVariant(button.Variant) {
		return fmt.Errorf("invalid button variant %q", button.Variant)
	}
	out.Static("<button")
	optionalAttribute(out, "id", button.ID)
	out.Static(` type="`)
	out.Attribute(string(buttonType))
	out.Static(`" `)
	buttonClass(out, button.Variant, button.Attrs)
	optionalAttribute(out, "aria-label", button.AriaLabel)
	if button.Disabled {
		out.Static(" disabled")
	}
	if button.Action.Method != 0 {
		if err := button.Action.RenderAttributes(out); err != nil {
			return err
		}
	}
	out.PassAttrs(button.Attrs, "type", when(button.ID, "id"),
		when(button.AriaLabel, "aria-label"), whenSet(button.Disabled, "disabled"),
		actionOwned(button.Action))
	out.Static(">")
	out.Text(button.Label)
	out.Static("</button>")
	return out.Err()
}

type Link struct {
	Label  string
	URL    string
	Action web.BrowserAction
	// Variant renders the link as a button (class "button button--<variant>").
	Variant ButtonVariant
	Attrs   html.Attrs
}

func (link Link) RenderHTML(_ context.Context, out *html.Context) error {
	if err := validateURL(link.URL); err != nil {
		return err
	}
	if !validButtonVariant(link.Variant) {
		return fmt.Errorf("invalid link variant %q", link.Variant)
	}
	out.Static("<a href=\"")
	out.Attribute(link.URL)
	out.Static("\"")
	if link.Variant != "" {
		out.Static(" ")
		buttonClass(out, link.Variant, link.Attrs)
	} else {
		out.Class("", link.Attrs)
	}
	if link.Action.Method != 0 {
		if err := link.Action.RenderAttributes(out); err != nil {
			return err
		}
	}
	out.PassAttrs(link.Attrs, "href", actionOwned(link.Action))
	out.Static(">")
	out.Text(link.Label)
	out.Static("</a>")
	return out.Err()
}
