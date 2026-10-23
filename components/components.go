package components

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dmundt/go-mvu/html"
)

type Empty struct{}

func (Empty) RenderHTML(context.Context, *html.Context) error { return nil }

type Text string

func (value Text) RenderHTML(_ context.Context, out *html.Context) error {
	out.Text(string(value))
	return out.Err()
}

type Sequence []html.Component

func (items Sequence) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Components(ctx, items)
	return out.Err()
}

// Paragraph renders escaped text in a <p> element.
type Paragraph string

func (value Paragraph) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<p>")
	out.Text(string(value))
	out.Static("</p>")
	return out.Err()
}

// Heading renders <h1> through <h6>. Level must be 1–6.
type Heading struct {
	Level int
	ID    string
	Text  string
	// Attrs adds passthrough attributes (class, data-*, aria-*, hx-*, ...).
	Attrs html.Attrs
}

func (heading Heading) RenderHTML(_ context.Context, out *html.Context) error {
	if heading.Level < 1 || heading.Level > 6 {
		return fmt.Errorf("invalid heading level %d", heading.Level)
	}
	tag := "h" + strconv.Itoa(heading.Level)
	out.Static("<" + tag)
	optionalAttribute(out, "id", heading.ID)
	out.Class("", heading.Attrs)
	out.PassAttrs(heading.Attrs, when(heading.ID, "id"))
	out.Static(">")
	out.Text(heading.Text)
	out.Static("</" + tag + ">")
	return out.Err()
}

// Output renders a computed value. Live announces changes to assistive
// technology, e.g. after an HTMX swap.
type Output struct {
	ID    string
	Value string
	Live  bool
	Attrs html.Attrs
}

func (output Output) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<output")
	optionalAttribute(out, "id", output.ID)
	if output.Live {
		out.Static(` aria-live="polite"`)
	}
	out.Class("", output.Attrs)
	out.PassAttrs(output.Attrs, when(output.ID, "id"), whenSet(output.Live, "aria-live"))
	out.Static(">")
	out.Text(output.Value)
	out.Static("</output>")
	return out.Err()
}

// when returns name when value is set, else "" (which owns nothing).
func when(value, name string) string {
	if value == "" {
		return ""
	}
	return name
}

// whenSet returns name when set is true, else "".
func whenSet(set bool, name string) string {
	if !set {
		return ""
	}
	return name
}

func optionalAttribute(out *html.Context, name, value string) {
	if value == "" {
		return
	}
	out.Static(" " + name + `="`)
	out.Attribute(value)
	out.Static(`"`)
}

type AlertKind string

const (
	AlertInfo    AlertKind = "info"
	AlertSuccess AlertKind = "success"
	AlertWarning AlertKind = "warning"
	AlertDanger  AlertKind = "danger"
)

type Alert struct {
	ID      string
	Kind    AlertKind
	Title   string
	Message string
	Attrs   html.Attrs
}

func (alert Alert) RenderHTML(_ context.Context, out *html.Context) error {
	kind := alert.Kind
	if kind == "" {
		kind = AlertInfo
	}
	if kind != AlertInfo && kind != AlertSuccess && kind != AlertWarning && kind != AlertDanger {
		return fmt.Errorf("invalid alert kind %q", kind)
	}
	out.Static("<div class=\"alert alert--")
	out.Attribute(string(kind))
	out.ExtraClass(alert.Attrs)
	out.Static("\" role=\"status\"")
	optionalAttribute(out, "id", alert.ID)
	out.PassAttrs(alert.Attrs, "role", when(alert.ID, "id"))
	out.Static(">")
	if alert.Title != "" {
		out.Static("<strong>")
		out.Text(alert.Title)
		out.Static("</strong> ")
	}
	out.Text(alert.Message)
	out.Static("</div>")
	return out.Err()
}

type BadgeKind string

type Badge struct {
	Kind  BadgeKind
	Label string
	Attrs html.Attrs
}

func (badge Badge) RenderHTML(_ context.Context, out *html.Context) error {
	if badge.Kind != "" && badge.Kind != "info" && badge.Kind != "success" && badge.Kind != "warning" && badge.Kind != "danger" {
		return fmt.Errorf("invalid badge kind %q", badge.Kind)
	}
	out.Static("<span class=\"badge")
	if badge.Kind != "" {
		out.Static(" badge--")
		out.Attribute(string(badge.Kind))
	}
	out.ExtraClass(badge.Attrs)
	out.Static("\"")
	out.PassAttrs(badge.Attrs)
	out.Static(">")
	out.Text(badge.Label)
	out.Static("</span>")
	return out.Err()
}

func validateURL(value string) error { return html.ValidateURL(value) }
