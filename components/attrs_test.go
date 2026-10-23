package components

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

func TestAttrsPassthroughMergesClassAndKeepsOwnedAttributes(t *testing.T) {
	extra := html.Attrs{
		html.A("class", "wide"), html.A("data-row", "7"), html.A("aria-describedby", "hint"),
		html.A("id", "ignored"),
	}
	cases := []struct {
		name      string
		component html.Component
		want      string
	}{
		{"card", Card{ID: "c", Body: Text("b"), Attrs: extra},
			`<article class="card card--default wide" id="c" data-row="7" aria-describedby="hint">`},
		{"card without id", Card{Body: Text("b"), Attrs: html.Attrs{html.A("id", "own")}},
			`<article class="card card--default" id="own">`},
		{"button", Button{Label: "Go", Variant: ButtonPrimary, Attrs: html.Attrs{
			html.A("class", "wide"), html.A("type", "submit"), html.A("hx-post", "/go"), html.Flag("autofocus"),
		}}, `<button type="button" class="button button--primary wide" hx-post="/go" autofocus>Go</button>`},
		{"button action owns hx", Button{Label: "Go", Action: web.BrowserAction{Method: web.Get, URL: "/a"},
			Attrs: html.Attrs{html.A("hx-get", "/b"), html.A("hx-target", "#x"), html.A("data-k", "v")}},
			`<button type="button" class="button" hx-get="/a" data-k="v">Go</button>`},
		{"heading", Heading{Level: 2, Text: "T", Attrs: html.Attrs{html.A("class", "muted")}},
			`<h2 class="muted">T</h2>`},
		{"link", Link{Label: "L", URL: "/l", Attrs: html.Attrs{html.A("href", "/evil"), html.A("rel", "next")}},
			`<a href="/l" rel="next">L</a>`},
		{"input", Input{Name: "q", Attrs: html.Attrs{html.A("class", "lg"), html.A("inputmode", "search"),
			html.A("name", "other")}},
			`<input class="input lg" id="q" name="q" type="text" value="" inputmode="search">`},
		{"stack", Stack{Attrs: html.Attrs{html.A("id", "s")}}, `<div class="stack" id="s"></div>`},
		{"main", Main{Attrs: html.Attrs{html.A("id", "content")}}, `<main id="content"></main>`},
		{"cell", TableCell{Header: true, Scope: ScopeRow, Attrs: html.Attrs{html.A("scope", "col"), html.A("colspan", "2")}},
			`<th scope="row" colspan="2"></th>`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := render(t, testCase.component); !strings.Contains(got, testCase.want) {
				t.Fatalf("got  %s\nwant %s", got, testCase.want)
			}
		})
	}
}

func TestAttrsPassthroughRejectsUnsafeAttributes(t *testing.T) {
	for _, component := range []html.Component{
		Card{Body: Text("b"), Attrs: html.Attrs{html.A("onclick", "x")}},
		Button{Label: "x", Attrs: html.Attrs{html.A("style", "color:red")}},
		Link{Label: "x", URL: "/", Attrs: html.Attrs{html.A("hx-get", "javascript:x")}},
		Input{Name: "q", Attrs: html.Attrs{html.A("hx-on:input", "x")}},
	} {
		err := html.Render(context.Background(), io.Discard, component)
		if !errors.Is(err, html.ErrUnsafeAttribute) {
			t.Errorf("%T: got %v, want ErrUnsafeAttribute", component, err)
		}
	}
}

func TestElementBuilderComposesWithComponents(t *testing.T) {
	action, err := web.BrowserAction{Method: web.Post, URL: "/save", Target: "#panel", Swap: web.OuterHTML}.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	got := render(t, html.El("section", html.Attrs{html.A("id", "panel")},
		Heading{Level: 2, Text: "Panel"},
		html.El("form", action, Button{Type: ButtonSubmit, Label: "Save"}),
	))
	want := `<section id="panel"><h2>Panel</h2><form hx-post="/save" hx-target="#panel" hx-swap="outerHTML">` +
		`<button type="submit" class="button">Save</button></form></section>`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestAttrsClassMergeAndDuplicateNames(t *testing.T) {
	attrs := html.Attrs{
		html.A("CLASS", "wide"), html.A("class", ""), html.Flag("class"),
		html.A("Class", `a"b`), html.A("data-row", "first"), html.A("DATA-ROW", "second"),
		html.A("id", "first-id"), html.A("ID", "second-id"),
	}

	got := render(t, Card{Attrs: attrs})
	want := `<article class="card card--default wide a&#34;b" data-row="first" id="first-id">` +
		`<div class="card__body"></div></article>`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestAttrsOwnedValuesAreDiscardedBeforeValidation(t *testing.T) {
	got := render(t, Link{URL: "/owned", Attrs: html.Attrs{
		html.A("HREF", "javascript:ignored"),
		html.A("data-note", "kept"),
	}})
	if got != `<a href="/owned" data-note="kept"></a>` {
		t.Fatalf("unexpected owned-attribute output: %s", got)
	}
}

func TestAttrsOptionalOwnedFields(t *testing.T) {
	idAttrs := html.Attrs{html.A("ID", "extra"), html.A("id", "duplicate")}
	for _, tc := range []struct {
		name      string
		component html.Component
		want      string
	}{
		{"card absent", Card{Attrs: idAttrs}, `<article class="card card--default" ID="extra">`},
		{"card set", Card{ID: "owned", Attrs: idAttrs}, `<article class="card card--default" id="owned">`},
		{"heading absent", Heading{Level: 2, Attrs: idAttrs}, `<h2 ID="extra">`},
		{"heading set", Heading{Level: 2, ID: "owned", Attrs: idAttrs}, `<h2 id="owned">`},
		{"section absent", Section{Attrs: idAttrs}, `<section ID="extra">`},
		{"section set", Section{ID: "owned", Attrs: idAttrs}, `<section id="owned">`},
		{"form absent", Form{URL: "/", Attrs: idAttrs}, `<form method="post" action="/" ID="extra">`},
		{"form set", Form{ID: "owned", URL: "/", Attrs: idAttrs}, `<form id="owned" method="post" action="/">`},
		{"button absent", Button{Attrs: idAttrs}, `<button type="button" class="button" ID="extra">`},
		{"button set", Button{ID: "owned", Attrs: idAttrs}, `<button id="owned" type="button" class="button">`},
		{"output absent", Output{Attrs: idAttrs}, `<output ID="extra">`},
		{"output set", Output{ID: "owned", Attrs: idAttrs}, `<output id="owned">`},
		{"button false flag", Button{Attrs: html.Attrs{html.Flag("DISABLED"), html.Flag("disabled")}},
			`<button type="button" class="button" DISABLED>`},
		{"button true flag", Button{Disabled: true, Attrs: html.Attrs{html.A("DISABLED", "false")}},
			`<button type="button" class="button" disabled>`},
		{"output not live", Output{Attrs: html.Attrs{html.A("ARIA-LIVE", "assertive")}},
			`<output ARIA-LIVE="assertive">`},
		{"output live", Output{Live: true, Attrs: html.Attrs{html.A("ARIA-LIVE", "off")}},
			`<output aria-live="polite">`},
		{"input defaults owned", Input{Name: "q", Attrs: html.Attrs{
			html.A("ID", "other"), html.A("NAME", "other"), html.A("TYPE", "email"),
			html.A("VALUE", "other"), html.A("PLACEHOLDER", "extra"),
		}}, `<input class="input" id="q" name="q" type="text" value="" PLACEHOLDER="extra">`},
		{"input optional owned", Input{Name: "q", Placeholder: "owned", Attrs: html.Attrs{
			html.A("PLACEHOLDER", "other"),
		}}, `<input class="input" id="q" name="q" type="text" value="" placeholder="owned">`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := render(t, tc.component)
			if !strings.HasPrefix(got, tc.want) {
				t.Fatalf("got %s, want prefix %s", got, tc.want)
			}
		})
	}
}

func TestAttrsActionOwnsEveryHTMXAlias(t *testing.T) {
	attrs := html.Attrs{
		html.A("HX-GET", "/other"), html.A("hx-post", "/other"), html.A("data-HX-PATCH", "/other"),
		html.A("HX-TARGET", "#other"), html.A("data-hx-target", "#alias"),
		html.A("hx-trigger", "every 2s"), html.A("data-hx-swap", "innerHTML"),
		html.A("data-note", "retained"),
	}
	action := web.BrowserAction{Method: web.Get, URL: "/owned", Target: "#owned"}
	for _, tc := range []struct {
		name      string
		component html.Component
	}{
		{"button", Button{Action: action, Attrs: attrs}},
		{"link", Link{URL: "/", Action: action, Attrs: attrs}},
		{"form", Form{URL: "/", Action: action, Attrs: attrs}},
		{"switcher", ThemeSwitcher{Attrs: attrs}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := render(t, tc.component)
			root := got[:strings.IndexByte(got, '>')]
			for _, forbidden := range []string{"/other", "#other", "#alias", "every 2s", "innerHTML"} {
				if strings.Contains(root, forbidden) {
					t.Fatalf("action conflict %q survived: %s", forbidden, root)
				}
			}
			if !strings.Contains(root, `data-note="retained"`) {
				t.Fatalf("unrelated data attribute dropped: %s", root)
			}
		})
	}
	for _, attrs := range []html.Attrs{
		{html.A("HX-GET", "/first"), html.A("data-hx-get", "/second")},
		{html.A("data-HX-get", "/first"), html.A("hx-get", "/second")},
	} {
		got := render(t, Button{Attrs: attrs})
		if !strings.Contains(got, `="/first"`) || strings.Contains(got, "/second") {
			t.Fatalf("first HTMX alias must win without Action: %s", got)
		}
	}
}
