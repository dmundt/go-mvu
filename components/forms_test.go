package components

import (
	"context"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

func pageLabels(rendered string) []string {
	var labels []string
	for _, part := range strings.Split(rendered, "</")[:] {
		index := strings.LastIndex(part, ">")
		if index < 0 || index == len(part)-1 {
			continue
		}
		labels = append(labels, part[index+1:])
	}
	return labels
}

func TestPaginationCases(t *testing.T) {
	cases := []struct {
		name       string
		pagination Pagination
		want       string
	}{
		{"zero items", Pagination{Page: 1, PageSize: 10}, ""},
		{"zero page size", Pagination{Page: 1, TotalItems: 10}, ""},
		{"one page", Pagination{Page: 1, PageSize: 10, TotalItems: 10}, ""},
		{"first", Pagination{Page: 1, PageSize: 10, TotalItems: 100}, "1 2 3 … 10 Next"},
		{"middle", Pagination{Page: 5, PageSize: 10, TotalItems: 100}, "Previous 1 … 3 4 5 6 7 … 10 Next"},
		{"last", Pagination{Page: 10, PageSize: 10, TotalItems: 100}, "Previous 1 … 8 9 10"},
		{"negative clamps to first", Pagination{Page: -4, PageSize: 10, TotalItems: 30}, "1 2 3 Next"},
		{"beyond clamps to last", Pagination{Page: 99, PageSize: 10, TotalItems: 30}, "Previous 1 2 3"},
		{"window one", Pagination{Page: 5, PageSize: 1, TotalItems: 9, Window: 1}, "Previous 1 … 4 5 6 … 9 Next"},
		{"no gap when adjacent", Pagination{Page: 3, PageSize: 1, TotalItems: 5, Window: 1}, "Previous 1 2 3 4 5 Next"},
		{"1000 pages", Pagination{Page: 500, PageSize: 1, TotalItems: 1000, Window: 1}, "Previous 1 … 499 500 501 … 1000 Next"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(pageLabels(render(t, tc.pagination)), " ")
			if got != tc.want {
				t.Fatalf("labels = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPaginationPreservesQueryAndMarksRel(t *testing.T) {
	rendered := render(t, Pagination{
		Page: 2, PageSize: 10, TotalItems: 30, BaseURL: "/users?q=a+b&sort=name", QueryName: "p",
	})
	for _, expected := range []string{
		`href="/users?p=1&amp;q=a+b&amp;sort=name" rel="prev"`,
		`href="/users?p=3&amp;q=a+b&amp;sort=name" rel="next"`,
		`aria-current="page">2</span>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("missing %q: %s", expected, rendered)
		}
	}
}

func TestPaginationCurrentAndPages(t *testing.T) {
	pagination := Pagination{Page: 0, PageSize: 7, TotalItems: 50}
	if pagination.Pages() != 8 || pagination.Current() != 1 {
		t.Fatalf("pages=%d current=%d", pagination.Pages(), pagination.Current())
	}
}

func TestFormRendersCSRFAndHTMX(t *testing.T) {
	rendered := render(t, Form{
		ID: "f", URL: "/users", CSRFField: Text("<csrf>"),
		Action:   web.BrowserAction{Method: web.Post, URL: "/users", Target: "#list"},
		Children: []html.Component{Label{For: "name", Text: "Name"}, FieldError{ID: "name-error", Message: "Required"}},
	})
	for _, expected := range []string{
		`<form id="f" method="post" action="/users" hx-post="/users" hx-target="#list">`,
		"&lt;csrf&gt;",
		`<label for="name">Name</label>`,
		`<span class="field-error" id="name-error" role="alert">Required</span>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("missing %q: %s", expected, rendered)
		}
	}
	get := render(t, Form{Method: FormGet, URL: "/search", CSRFField: Text("csrf")})
	if strings.Contains(get, "csrf") {
		t.Fatalf("GET form leaked CSRF token: %s", get)
	}
	var output strings.Builder
	if err := html.Render(context.Background(), &output, Form{Method: "delete", URL: "/"}); err == nil {
		t.Fatal("accepted invalid method")
	}
	if err := html.Render(context.Background(), &output, Form{URL: "javascript:x"}); err == nil {
		t.Fatal("accepted unsafe URL")
	}
}

func TestFieldErrorEmptyRendersNothing(t *testing.T) {
	if got := render(t, FieldError{ID: "x"}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestTableParts(t *testing.T) {
	rendered := render(t, Table{
		Header: HeaderRow{"Name", "<Role>"},
		Body: TableRow{ID: "u1", Cells: []html.Component{
			TableCell{Header: true, Scope: ScopeRow, Body: Text("Ada")},
			TableCell{Body: Text("Admin")},
		}},
	})
	want := `<div class="table-scroll"><table><thead><tr><th scope="col">Name</th><th scope="col">&lt;Role&gt;</th></tr></thead>` +
		`<tbody><tr id="u1"><th scope="row">Ada</th><td>Admin</td></tr></tbody></table></div>`
	if rendered != want {
		t.Fatalf("got  %s\nwant %s", rendered, want)
	}
	var output strings.Builder
	if err := html.Render(context.Background(), &output, TableCell{Scope: "bad"}); err == nil {
		t.Fatal("accepted invalid scope")
	}
}

func TestDocumentRendersHTMXIntegrity(t *testing.T) {
	rendered := render(t, Document{HTMXSource: "/static/htmx.min.js", HTMXIntegrity: "sha384-abc"})
	if !strings.Contains(rendered, `<script src="/static/htmx.min.js" integrity="sha384-abc" defer></script>`) {
		t.Fatalf("missing integrity: %s", rendered)
	}
	if !strings.Contains(rendered, `data-theme="system"`) {
		t.Fatalf("default mode should be system: %s", rendered)
	}
}
