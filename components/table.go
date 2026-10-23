package components

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/html"
)

// Table renders a semantic table inside a horizontally scrollable wrapper.
// Header, Body, and Footer are wrapped in thead/tbody/tfoot. Sections, when
// set, are rendered as-is instead (use TableHeader and TableBody).
type Table struct {
	ID       string
	Caption  string
	Header   html.Component
	Body     html.Component
	Footer   html.Component
	Sections []html.Component
	// Attrs apply to the <table> element.
	Attrs html.Attrs
}

func (table Table) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Static("<div class=\"table-scroll\"><table")
	optionalAttribute(out, "id", table.ID)
	out.Class("", table.Attrs)
	out.PassAttrs(table.Attrs, when(table.ID, "id"))
	out.Static(">")
	if table.Caption != "" {
		out.Static("<caption>")
		out.Text(table.Caption)
		out.Static("</caption>")
	}
	if table.Sections != nil {
		out.Components(ctx, table.Sections)
		out.Static("</table></div>")
		return out.Err()
	}
	if table.Header != nil {
		out.Static("<thead>")
		out.Component(ctx, table.Header)
		out.Static("</thead>")
	}
	if table.Body != nil {
		out.Static("<tbody>")
		out.Component(ctx, table.Body)
		out.Static("</tbody>")
	}
	if table.Footer != nil {
		out.Static("<tfoot>")
		out.Component(ctx, table.Footer)
		out.Static("</tfoot>")
	}
	out.Static("</table></div>")
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
