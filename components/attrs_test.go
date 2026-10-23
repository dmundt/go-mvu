package components

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
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
		{"button action owns hx", Button{Label: "Go", Action: htmx.Action{Method: htmx.Get, URL: "/a"},
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
	action, err := htmx.Action{Method: htmx.Post, URL: "/save", Target: "#panel", Swap: htmx.OuterHTML}.Attrs()
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
