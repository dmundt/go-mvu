package components

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/html"
)

// AppShell is the spec name for Shell.
type AppShell = Shell

// Shell lays out header, optional sidebar, main and footer landmarks.
type Shell struct {
	Header  html.Component
	Sidebar html.Component
	Main    html.Component
	Footer  html.Component
	// Attrs apply to the div.shell wrapper.
	Attrs html.Attrs
}

func (shell Shell) RenderHTML(ctx context.Context, out *html.Context) error {
	if shell.Header != nil {
		out.Static("<header>")
		out.Component(ctx, shell.Header)
		out.Static("</header>")
	}
	out.Static(`<div class="shell`)
	out.ExtraClass(shell.Attrs)
	out.Static(`"`)
	out.PassAttrs(shell.Attrs)
	out.Static(">")
	if shell.Sidebar != nil {
		out.Static("<aside>")
		out.Component(ctx, shell.Sidebar)
		out.Static("</aside>")
	}
	out.Static("<main>")
	out.Component(ctx, shell.Main)
	out.Static("</main></div>")
	if shell.Footer != nil {
		out.Static("<footer>")
		out.Component(ctx, shell.Footer)
		out.Static("</footer>")
	}
	return out.Err()
}

type Stack struct {
	Gap      string
	Children []html.Component
	Attrs    html.Attrs
}

func (stack Stack) RenderHTML(ctx context.Context, out *html.Context) error {
	class := "stack"
	if stack.Gap != "" {
		gap, ok := gapToken(stack.Gap)
		if !ok {
			return fmt.Errorf("invalid stack gap %q", stack.Gap)
		}
		class += " stack--gap-" + gap
	}
	out.Static(`<div class="`)
	out.Static(class)
	out.ExtraClass(stack.Attrs)
	out.Static(`"`)
	out.PassAttrs(stack.Attrs)
	out.Static(">")
	out.Components(ctx, stack.Children)
	out.Static("</div>")
	return out.Err()
}

type Grid struct {
	Columns  int
	Gap      string
	Children []html.Component
	Attrs    html.Attrs
}

func (grid Grid) RenderHTML(ctx context.Context, out *html.Context) error {
	columns := grid.Columns
	if columns < 1 {
		columns = 1
	}
	if columns > 12 {
		return fmt.Errorf("grid columns %d exceeds maximum of 12", columns)
	}
	gap := "md"
	if grid.Gap != "" {
		var ok bool
		gap, ok = gapToken(grid.Gap)
		if !ok {
			return fmt.Errorf("invalid grid gap %q", grid.Gap)
		}
	}
	out.Static(`<div class="grid grid--columns-`)
	out.Int(columns)
	out.Static(" grid--gap-")
	out.Static(gap)
	out.ExtraClass(grid.Attrs)
	out.Static(`"`)
	out.PassAttrs(grid.Attrs)
	out.Static(">")
	out.Components(ctx, grid.Children)
	out.Static("</div>")
	return out.Err()
}

type CardVariant string

const (
	CardDefault CardVariant = "default"
	CardPrimary CardVariant = "primary"
	CardSuccess CardVariant = "success"
	CardWarning CardVariant = "warning"
	CardDanger  CardVariant = "danger"
)

type Card struct {
	ID      string
	Header  html.Component
	Body    html.Component
	Footer  html.Component
	Variant CardVariant
	Attrs   html.Attrs
}

func (card Card) RenderHTML(ctx context.Context, out *html.Context) error {
	variant := card.Variant
	if variant == "" {
		variant = CardDefault
	}
	if variant != CardDefault && variant != CardPrimary && variant != CardSuccess && variant != CardWarning && variant != CardDanger {
		return fmt.Errorf("invalid card variant %q", variant)
	}
	out.Static("<article class=\"card card--")
	out.Attribute(string(variant))
	out.ExtraClass(card.Attrs)
	out.Static("\"")
	optionalAttribute(out, "id", card.ID)
	out.PassAttrs(card.Attrs, when(card.ID, "id"))
	out.Static(">")
	if card.Header != nil {
		out.Static("<header class=\"card__header\">")
		out.Component(ctx, card.Header)
		out.Static("</header>")
	}
	out.Static("<div class=\"card__body\">")
	out.Component(ctx, card.Body)
	out.Static("</div>")
	if card.Footer != nil {
		out.Static("<footer class=\"card__footer\">")
		out.Component(ctx, card.Footer)
		out.Static("</footer>")
	}
	out.Static("</article>")
	return out.Err()
}

func gapToken(value string) (string, bool) {
	switch value {
	case "0", "0px":
		return "none", true
	case "0.25rem", "4px":
		return "xs", true
	case "0.5rem", "8px":
		return "sm", true
	case "1rem", "16px":
		return "md", true
	case "1.5rem", "24px":
		return "lg", true
	case "2rem", "32px":
		return "xl", true
	default:
		return "", false
	}
}

// Column arranges children vertically. Use Stack for custom gaps or attributes.
func Column(children ...html.Component) Stack {
	return Stack{Children: children}
}

// Row arranges children horizontally with wrapping. Use Inline for attributes.
func Row(children ...html.Component) Inline {
	return Inline{Children: children}
}

// FormLayout stacks labels, controls and feedback with full-width inputs.
func FormLayout(children ...html.Component) Stack {
	return Stack{
		Children: children,
		Attrs:    html.Attrs{html.A("class", "form-fields")},
	}
}
