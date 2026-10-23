package components

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
)

type classTheme struct{ theme.DefaultTheme }

func (classTheme) DocumentClass(mode theme.Mode) string { return "t-" + mode.String() }

func TestLandmarksRenderSemanticElements(t *testing.T) {
	cases := []struct {
		component html.Component
		want      string
	}{
		{Header{Children: []html.Component{Text("h")}}, "<header>h</header>"},
		{Sidebar{AriaLabel: "Filters <x>", Children: []html.Component{Text("s")}},
			`<aside aria-label="Filters &lt;x&gt;">s</aside>`},
		{Main{Children: []html.Component{Text("m")}}, "<main>m</main>"},
		{Footer{}, "<footer></footer>"},
		{Inline{Children: []html.Component{Text("a"), Text("b")}}, `<div class="inline">ab</div>`},
	}
	for _, c := range cases {
		if got := render(t, c.component); got != c.want {
			t.Errorf("%T = %q, want %q", c.component, got, c.want)
		}
	}
}

func TestEmptyStateEscapesAndRequiresTitle(t *testing.T) {
	got := render(t, EmptyState{Title: "None <b>", Description: "d"})
	want := `<div class="empty-state"><p class="empty-state__title">None &lt;b&gt;</p>` +
		`<p class="empty-state__description">d</p></div>`
	if got != want {
		t.Fatalf("got %q", got)
	}
	if err := html.Render(context.Background(), io.Discard, EmptyState{}); err == nil {
		t.Fatal("empty title accepted")
	}
}

func TestCheckboxAndTextareaEscapeAndAssociateErrors(t *testing.T) {
	checkbox := render(t, Checkbox{Name: "agree", Label: "Agree & <go>", Checked: true, Required: true, Error: "Need"})
	for _, want := range []string{
		`id="agree" name="agree" value="on" checked required aria-invalid="true" aria-describedby="agree-error"`,
		"Agree &amp; &lt;go&gt;", `id="agree-error">Need</span>`,
	} {
		if !strings.Contains(checkbox, want) {
			t.Errorf("checkbox missing %q in %q", want, checkbox)
		}
	}
	textarea := render(t, Textarea{Name: "bio", Label: "Bio", Value: "</textarea><b>", Hint: "short", Error: "bad"})
	for _, want := range []string{
		`aria-describedby="bio-hint bio-error"`, "&lt;/textarea&gt;&lt;b&gt;</textarea>", `id="bio-hint">short`,
	} {
		if !strings.Contains(textarea, want) {
			t.Errorf("textarea missing %q in %q", want, textarea)
		}
	}
	for _, invalid := range []html.Component{
		Checkbox{Name: "a"}, Textarea{Name: "a", Label: "b", Rows: 99}, Textarea{Name: "a", Label: "b", ID: "bad id"},
	} {
		if err := html.Render(context.Background(), io.Discard, invalid); err == nil {
			t.Errorf("%#v accepted", invalid)
		}
	}
}

func TestTableSectionsAndScrollWrapper(t *testing.T) {
	got := render(t, Table{Sections: []html.Component{
		TableHeader{Rows: []html.Component{Text("hr")}},
		TableBody{Rows: []html.Component{Text("br")}},
	}})
	want := `<div class="table-scroll"><table><thead>hr</thead><tbody>br</tbody></table></div>`
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestThemeSwitcherActionAndTarget(t *testing.T) {
	got := render(t, ThemeSwitcher{Mode: theme.Dark, Action: "/prefs", Target: "#app"})
	for _, want := range []string{
		`action="/prefs"`, `hx-post="/prefs"`, `hx-target="#app"`, `hx-swap="outerHTML"`,
		`value="dark" class="button button--primary" aria-pressed="true"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if got := render(t, ThemeSwitcher{}); !strings.Contains(got, `hx-swap="none"`) || strings.Contains(got, "hx-target") {
		t.Errorf("default switcher = %q", got)
	}
}

func TestPaginationCustomLabels(t *testing.T) {
	got := render(t, Pagination{Page: 2, PageSize: 10, TotalItems: 50, BaseURL: "/u", PreviousLabel: "Back", NextLabel: "Fwd"})
	if !strings.Contains(got, `rel="prev">Back</a>`) || !strings.Contains(got, `rel="next">Fwd</a>`) {
		t.Fatalf("got %q", got)
	}
}

func TestDocumentClassFromDocumentClasser(t *testing.T) {
	got := render(t, Document{Title: "x", Theme: classTheme{}, Mode: theme.Dark})
	if !strings.Contains(got, `data-theme-name="default" class="t-dark">`) {
		t.Fatalf("got %q", got[:120])
	}
	if got := render(t, Document{Title: "x"}); strings.Contains(got[:120], " class=") {
		t.Fatalf("default theme emitted class: %q", got[:120])
	}
}
