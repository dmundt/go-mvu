package components

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dmundt/go-mvu/html"
)

// NavigationItem is the spec name for NavItem.
type NavigationItem = NavItem

// BreadcrumbItem is the spec name for Breadcrumb.
type BreadcrumbItem = Breadcrumb

// Header renders a <header> landmark. Use inside Shell slots or standalone.
type Header struct{ Children []html.Component }

func (header Header) RenderHTML(ctx context.Context, out *html.Context) error {
	return landmark(ctx, out, "header", "", header.Children)
}

// Sidebar renders an <aside> landmark with an optional accessible label.
type Sidebar struct {
	AriaLabel string
	Children  []html.Component
}

func (sidebar Sidebar) RenderHTML(ctx context.Context, out *html.Context) error {
	return landmark(ctx, out, "aside", sidebar.AriaLabel, sidebar.Children)
}

// Main renders the <main> landmark.
type Main struct{ Children []html.Component }

func (main Main) RenderHTML(ctx context.Context, out *html.Context) error {
	return landmark(ctx, out, "main", "", main.Children)
}

// Footer renders a <footer> landmark.
type Footer struct{ Children []html.Component }

func (footer Footer) RenderHTML(ctx context.Context, out *html.Context) error {
	return landmark(ctx, out, "footer", "", footer.Children)
}

func landmark(
	ctx context.Context, out *html.Context, tag, label string, children []html.Component,
) error {
	out.Static("<")
	out.Static(tag)
	optionalAttribute(out, "aria-label", label)
	out.Static(">")
	out.Components(ctx, children)
	out.Static("</")
	out.Static(tag)
	out.Static(">")
	return out.Err()
}

// Inline arranges children horizontally and wraps them on narrow screens.
type Inline struct {
	Children []html.Component
}

func (inline Inline) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Static(`<div class="inline">`)
	out.Components(ctx, inline.Children)
	out.Static("</div>")
	return out.Err()
}

// EmptyState explains why content is absent and offers an optional action.
type EmptyState struct {
	Title       string
	Description string
	Action      html.Component
}

func (empty EmptyState) RenderHTML(ctx context.Context, out *html.Context) error {
	if empty.Title == "" {
		return errors.New("components: empty state requires title")
	}
	out.Static(`<div class="empty-state"><p class="empty-state__title">`)
	out.Text(empty.Title)
	out.Static("</p>")
	if empty.Description != "" {
		out.Static(`<p class="empty-state__description">`)
		out.Text(empty.Description)
		out.Static("</p>")
	}
	if empty.Action != nil {
		out.Static(`<div class="empty-state__action">`)
		out.Component(ctx, empty.Action)
		out.Static("</div>")
	}
	out.Static("</div>")
	return out.Err()
}

// Checkbox renders a labelled checkbox with optional validation feedback.
type Checkbox struct {
	ID       string
	Name     string
	Label    string
	Value    string
	Checked  bool
	Disabled bool
	Required bool
	Error    string
}

func (checkbox Checkbox) RenderHTML(_ context.Context, out *html.Context) error {
	id, err := controlID(checkbox.ID, checkbox.Name, checkbox.Label)
	if err != nil {
		return err
	}
	value := checkbox.Value
	if value == "" {
		value = "on"
	}
	out.Static(`<div class="choice"><label class="checkbox"><input type="checkbox"`)
	attribute(out, "id", id)
	attribute(out, "name", checkbox.Name)
	attribute(out, "value", value)
	flag(out, "checked", checkbox.Checked)
	flag(out, "disabled", checkbox.Disabled)
	flag(out, "required", checkbox.Required)
	if checkbox.Error != "" {
		out.Static(` aria-invalid="true"`)
		attribute(out, "aria-describedby", id+"-error")
	}
	out.Static("><span>")
	out.Text(checkbox.Label)
	out.Static("</span></label>")
	controlError(out, id+"-error", checkbox.Error)
	out.Static("</div>")
	return out.Err()
}

// Textarea renders a labelled multi-line text field with hint and error text.
type Textarea struct {
	ID          string
	Name        string
	Label       string
	Value       string
	Placeholder string
	Hint        string
	Rows        int
	Required    bool
	Disabled    bool
	Error       string
}

func (textarea Textarea) RenderHTML(_ context.Context, out *html.Context) error {
	id, err := controlID(textarea.ID, textarea.Name, textarea.Label)
	if err != nil {
		return err
	}
	rows := textarea.Rows
	if rows == 0 {
		rows = 4
	}
	if rows < 1 || rows > 50 {
		return fmt.Errorf("components: textarea rows %d outside 1..50", textarea.Rows)
	}
	hintID, errorID := "", ""
	if textarea.Hint != "" {
		hintID = id + "-hint"
	}
	if textarea.Error != "" {
		errorID = id + "-error"
	}
	out.Static(`<div class="form-field"><label`)
	attribute(out, "for", id)
	out.Static(">")
	out.Text(textarea.Label)
	out.Static(`</label><textarea class="input textarea"`)
	attribute(out, "id", id)
	attribute(out, "name", textarea.Name)
	out.Static(` rows="`)
	out.Int(rows)
	out.Static(`"`)
	optionalAttribute(out, "placeholder", textarea.Placeholder)
	flag(out, "required", textarea.Required)
	flag(out, "disabled", textarea.Disabled)
	if errorID != "" {
		out.Static(` aria-invalid="true"`)
	}
	optionalAttribute(out, "aria-describedby", joinIDs(hintID, errorID))
	// The newline preserves a leading newline in Value, which parsers strip.
	out.Static(">\n")
	out.Text(textarea.Value)
	out.Static("</textarea>")
	if hintID != "" {
		out.Static(`<small class="form-field__hint"`)
		attribute(out, "id", hintID)
		out.Static(">")
		out.Text(textarea.Hint)
		out.Static("</small>")
	}
	controlError(out, errorID, textarea.Error)
	out.Static("</div>")
	return out.Err()
}

// TableHeader renders <thead> around rows. Use in Table.Sections.
type TableHeader struct{ Rows []html.Component }

func (header TableHeader) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Static("<thead>")
	out.Components(ctx, header.Rows)
	out.Static("</thead>")
	return out.Err()
}

// TableBody renders <tbody> around rows. Use in Table.Sections.
type TableBody struct{ Rows []html.Component }

func (body TableBody) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Static("<tbody>")
	out.Components(ctx, body.Rows)
	out.Static("</tbody>")
	return out.Err()
}

func controlID(id, name, label string) (string, error) {
	if id == "" {
		id = name
	}
	if err := validID(id); err != nil {
		return "", err
	}
	if name == "" || label == "" {
		return "", errors.New("components: form control requires name and label")
	}
	return id, nil
}

func validID(id string) error {
	if id == "" {
		return errors.New("components: missing ID")
	}
	for index, r := range id {
		letter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if letter || index > 0 && (r >= '0' && r <= '9' || r == '-' || r == '_') {
			continue
		}
		return fmt.Errorf("components: invalid ID %q", id)
	}
	return nil
}

func attribute(out *html.Context, name, value string) {
	out.Static(" ")
	out.Static(name)
	out.Static(`="`)
	out.Attribute(value)
	out.Static(`"`)
}

func flag(out *html.Context, name string, enabled bool) {
	if enabled {
		out.Static(" ")
		out.Static(name)
	}
}

func controlError(out *html.Context, id, message string) {
	if message == "" {
		return
	}
	out.Static(`<span class="field-error" role="alert" id="`)
	out.Attribute(id)
	out.Static(`">`)
	out.Text(message)
	out.Static("</span>")
}

func joinIDs(ids ...string) string {
	present := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			present = append(present, id)
		}
	}
	return strings.Join(present, " ")
}
