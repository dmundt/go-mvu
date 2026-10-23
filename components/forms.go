package components

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
)

// FormMethod is GET or POST; HTML forms support no others.
type FormMethod string

const (
	FormGet  FormMethod = "get"
	FormPost FormMethod = "post"
)

// Form renders <form> with optional CSRF field and HTMX action.
// Without Action.Method the form submits as plain HTML.
type Form struct {
	ID        string
	Method    FormMethod
	URL       string
	CSRFField html.Component
	Action    htmx.Action
	Children  []html.Component
	Attrs     html.Attrs
}

func (form Form) RenderHTML(ctx context.Context, out *html.Context) error {
	method := form.Method
	if method == "" {
		method = FormPost
	}
	if method != FormGet && method != FormPost {
		return fmt.Errorf("invalid form method %q", method)
	}
	if err := validateURL(form.URL); err != nil {
		return fmt.Errorf("invalid form URL: %w", err)
	}
	out.Static(`<form`)
	optionalAttribute(out, "id", form.ID)
	out.Static(` method="` + string(method) + `" action="`)
	out.Attribute(form.URL)
	out.Static(`"`)
	out.Class("", form.Attrs)
	if form.Action.Method != 0 {
		if err := form.Action.RenderAttributes(out); err != nil {
			return err
		}
	}
	out.PassAttrs(form.Attrs, when(form.ID, "id"), "method", "action", actionOwned(form.Action))
	out.Static(">")
	if method == FormPost {
		out.Component(ctx, form.CSRFField)
	}
	for _, child := range form.Children {
		out.Component(ctx, child)
	}
	out.Static("</form>")
	return out.Err()
}

// Label renders <label for=…>.
type Label struct {
	For   string
	Text  string
	Attrs html.Attrs
}

func (label Label) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<label")
	optionalAttribute(out, "for", label.For)
	out.Class("", label.Attrs)
	out.PassAttrs(label.Attrs, when(label.For, "for"))
	out.Static(">")
	out.Text(label.Text)
	out.Static("</label>")
	return out.Err()
}

// FieldError renders an error message linked via aria-describedby.
// Empty Message renders nothing.
type FieldError struct {
	ID      string
	Message string
	Attrs   html.Attrs
}

func (fieldError FieldError) RenderHTML(_ context.Context, out *html.Context) error {
	if fieldError.Message == "" {
		return nil
	}
	out.Static(`<span class="field-error`)
	out.ExtraClass(fieldError.Attrs)
	out.Static(`"`)
	optionalAttribute(out, "id", fieldError.ID)
	out.Static(` role="alert"`)
	out.PassAttrs(fieldError.Attrs, "role", when(fieldError.ID, "id"))
	out.Static(">")
	out.Text(fieldError.Message)
	out.Static("</span>")
	return out.Err()
}

// TableRow renders <tr> around cells.
type TableRow struct {
	ID    string
	Cells []html.Component
	Attrs html.Attrs
}

func (row TableRow) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Static("<tr")
	optionalAttribute(out, "id", row.ID)
	if len(row.Attrs) != 0 {
		out.Class("", row.Attrs)
		out.PassAttrs(row.Attrs, when(row.ID, "id"))
	}
	out.Static(">")
	for _, cell := range row.Cells {
		out.Component(ctx, cell)
	}
	out.Static("</tr>")
	return out.Err()
}

// CellScope sets <th scope>.
type CellScope string

const (
	ScopeColumn CellScope = "col"
	ScopeRow    CellScope = "row"
)

// TableCell renders <td>, or <th> when Header is true.
type TableCell struct {
	Header bool
	Scope  CellScope
	Body   html.Component
	Attrs  html.Attrs
}

func (cell TableCell) RenderHTML(ctx context.Context, out *html.Context) error {
	if cell.Scope != "" && cell.Scope != ScopeColumn && cell.Scope != ScopeRow {
		return fmt.Errorf("invalid table cell scope %q", cell.Scope)
	}
	// open ends with ">" so the common no-Attrs path is a single write.
	open, closing := "<td>", "</td>"
	if cell.Header {
		open, closing = "<th>", "</th>"
		switch cell.Scope {
		case ScopeColumn:
			open = `<th scope="col">`
		case ScopeRow:
			open = `<th scope="row">`
		}
	}
	if len(cell.Attrs) == 0 {
		out.Static(open)
	} else {
		out.Static(open[:len(open)-1])
		out.Class("", cell.Attrs)
		out.PassAttrs(cell.Attrs, whenSet(cell.Header && cell.Scope != "", "scope"))
		out.Static(">")
	}
	out.Component(ctx, cell.Body)
	out.Static(closing)
	return out.Err()
}

// HeaderRow renders a <tr> of <th scope="col"> labels.
type HeaderRow []string

func (labels HeaderRow) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<tr>")
	for _, label := range labels {
		out.Static(`<th scope="col">`)
		out.Text(label)
		out.Static("</th>")
	}
	out.Static("</tr>")
	return out.Err()
}
