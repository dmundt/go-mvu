package components

import (
	"context"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
)

func TestPaginationRendersAccessibleHTMXLinksAndEscapesURL(t *testing.T) {
	pagination := Pagination{
		Page: 2, PageSize: 10, TotalItems: 30, BaseURL: "/users?filter=a%26b",
		Target: "#results", PushURL: true, Window: 1,
	}
	var output strings.Builder
	err := html.Render(context.Background(), &output, pagination)
	if err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	for _, expected := range []string{
		`aria-label="Pagination"`,
		`href="/users?filter=a%26b&amp;page=1"`,
		`hx-get="/users?filter=a%26b&amp;page=1"`,
		`hx-push-url="true"`,
		`aria-current="page">2</span>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("rendered pagination missing %q: %s", expected, rendered)
		}
	}
}

func TestLinkRejectsUnsafeURL(t *testing.T) {
	var output strings.Builder
	err := html.Render(context.Background(), &output, Link{Label: "unsafe", URL: "javascript:alert(1)"})
	if err == nil {
		t.Fatal("render accepted a javascript URL")
	}
}

func TestDocumentEmbedsThemeCSSWithoutExternalStylesheets(t *testing.T) {
	var output strings.Builder
	err := html.Render(context.Background(), &output, Document{
		Title: "Inline", Theme: theme.Default,
	})
	if err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	if !strings.Contains(rendered, "<style>") || !strings.Contains(rendered, ".card--primary") {
		t.Fatalf("document missing embedded theme stylesheet: %s", rendered)
	}
	if !strings.Contains(rendered, `data-theme-name="default"`) {
		t.Fatal("document missing theme identity for scoped presentation")
	}
	if strings.Contains(rendered, `rel="stylesheet"`) || strings.Contains(rendered, `style="`) {
		t.Fatalf("document emitted external or inline-attribute CSS: %s", rendered)
	}
}

type namedTheme string

func (name namedTheme) Name() string       { return string(name) }
func (name namedTheme) Stylesheet() string { return "" }

func TestDocumentEscapesThemeIdentity(t *testing.T) {
	var output strings.Builder
	if err := html.Render(context.Background(), &output, Document{
		Theme: namedTheme(`custom" onload="bad`),
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `data-theme-name="custom&#34; onload=&#34;bad"`) {
		t.Fatalf("theme name was not escaped: %s", output.String())
	}
}

func TestDocumentDefaultsThemeIdentity(t *testing.T) {
	var output strings.Builder
	if err := html.Render(context.Background(), &output, Document{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `data-theme-name="default"`) {
		t.Fatal("implicit default theme missing identity")
	}
}

func TestLayoutGapsUseThemeClassesNotStyleAttributes(t *testing.T) {
	for _, component := range []html.Component{
		Stack{Gap: "0.5rem"},
		Grid{Columns: 3, Gap: "1.5rem"},
	} {
		var output strings.Builder
		if err := html.Render(context.Background(), &output, component); err != nil {
			t.Fatal(err)
		}
		rendered := output.String()
		if strings.Contains(rendered, " style=") {
			t.Errorf("component emitted inline style attribute: %s", rendered)
		}
		if !strings.Contains(rendered, "class=") {
			t.Errorf("component missing theme class: %s", rendered)
		}
	}
}

func TestDefaultThemeStylesEveryEmittedComponentVariant(t *testing.T) {
	variants := map[string][]string{
		".card--":   {"default", "primary", "success", "warning", "danger"},
		".alert--":  {"info", "success", "warning", "danger"},
		".badge--":  {"info", "success", "warning", "danger"},
		".button--": {"primary", "secondary", "danger", "ghost"},
	}
	css := theme.Default.Stylesheet()
	for selector, kinds := range variants {
		for _, kind := range kinds {
			if !strings.Contains(css, selector+kind) {
				t.Errorf("default theme missing style selector %q", selector+kind)
			}
		}
	}
}

func TestSelectValidationErrorHasAccessibleAssociation(t *testing.T) {
	var output strings.Builder
	err := html.Render(context.Background(), &output, Select{
		Name: "email", Label: "Email", Error: "Invalid address",
	})
	if err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	for _, expected := range []string{
		`id="email"`,
		`aria-invalid="true"`,
		`aria-describedby="email-error"`,
		`id="email-error"`,
		`role="alert"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("select error markup missing %q: %s", expected, rendered)
		}
	}
}

func TestThemeSwitcherOffersThreeModesForNormalAndHTMXPost(t *testing.T) {
	var output strings.Builder
	err := html.Render(context.Background(), &output, ThemeSwitcher{
		Mode: theme.Dark, CSRFField: Text("csrf"), Return: "/users?page=2",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`method="post"`,
		`action="/theme"`,
		`hx-post="/theme"`,
		`hx-swap="none"`,
		`name="return" value="/users?page=2"`,
		`name="mode" value="system" class="button button--secondary" aria-pressed="false">System`,
		`name="mode" value="light" class="button button--secondary" aria-pressed="false">Light`,
		`name="mode" value="dark" class="button button--primary" aria-pressed="true">Dark`,
		"csrf",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("theme switcher missing %q: %s", expected, output.String())
		}
	}
}

func TestThemeSwitcherDropsUnsafeReturn(t *testing.T) {
	var output strings.Builder
	if err := html.Render(context.Background(), &output, ThemeSwitcher{Return: "//evil.test"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "evil") {
		t.Fatalf("unsafe return rendered: %s", output.String())
	}
}
