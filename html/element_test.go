package html

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestElementBuilderRendersNestedEscapedMarkup(t *testing.T) {
	got := render(t, El("section", Attrs{A("class", "panel"), A("data-id", `7"x`), Flag("hidden")},
		El("h2", nil, Text("A<B")),
		El("img", Attrs{A("src", "/a.png"), A("alt", "")}),
		El("a", Attrs{A("href", "https://example.com/?a=1&b=2")}, Text("go")),
	))
	want := `<section class="panel" data-id="7&#34;x" hidden><h2>A&lt;B</h2>` +
		`<img src="/a.png" alt=""><a href="https://example.com/?a=1&amp;b=2">go</a></section>`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestElementRejectsInvalidTagsAndVoidChildren(t *testing.T) {
	for _, element := range []Element{
		El("", nil), El("Div", nil), El("1a", nil), El("a b", nil), El("x>", nil),
		El("script", nil), El("style", nil), El("iframe", nil), El("link", nil),
		El("br", nil, Text("x")),
	} {
		err := Render(context.Background(), io.Discard, element)
		if !errors.Is(err, ErrInvalidElement) {
			t.Errorf("%q: got %v, want ErrInvalidElement", element.Tag, err)
		}
	}
	if err := Render(context.Background(), io.Discard, El("my-widget", nil)); err != nil {
		t.Fatalf("custom element: %v", err)
	}
}

func TestAttributeAllowlist(t *testing.T) {
	rejected := []Attr{
		A("onclick", "x"), A("ONLOAD", "x"), A("style", "color:red"), A("srcdoc", "<p>"),
		A("hx-on:click", "x"), A("hx-on-click", "x"), A("data-hx-on:click", "x"), A("hx-vars", "a:1"),
		A("hx-vals", "js:{a:1}"), A("data-hx-headers", " javascript:x"),
		A("hx-trigger", "click[ctrlKey]"),
		A("href", "javascript:alert(1)"), A("href", " JavaScript:x"), A("src", "data:text/html,x"),
		A("formaction", "vbscript:x"), A("hx-get", "javascript:x"), A("hx-push-url", "javascript:x"),
		A("", "x"), A("a b", "x"), A(`a"`, "x"), A("a=b", "x"), A("-x", "x"), Flag("onclick"),
	}
	for _, attr := range rejected {
		err := Render(context.Background(), io.Discard, El("div", Attrs{attr}))
		if !errors.Is(err, ErrUnsafeAttribute) {
			t.Errorf("%q=%q: got %v, want ErrUnsafeAttribute", attr.Name, attr.Value, err)
		}
	}
	accepted := Attrs{
		A("id", "x"), A("aria-label", "x"), A("data-row", "1"), A("hx-post", "/save"),
		A("hx-target", "#x"), A("hx-vals", `{"a":1}`), A("hx-push-url", "true"),
		A("href", "/a"), A("href", "mailto:a@b.c"), A("xlink:href", "#icon"),
		A("title", `"><script>`), Flag("disabled"),
	}
	got := render(t, El("div", accepted))
	if !strings.Contains(got, `title="&#34;&gt;&lt;script&gt;"`) || !strings.Contains(got, " disabled>") {
		t.Fatalf("unexpected output %s", got)
	}
}

func TestPassAttrsMergesClassAndSkipsOwned(t *testing.T) {
	attrs := Attrs{
		A("class", "wide"), A("id", "user"), A("type", "submit"), A("hx-get", "/x"),
		A("CLASS", "dark"), A("data-x", "1"),
	}
	got := render(t, ComponentFunc(func(_ context.Context, out *Context) error {
		out.Static(`<button type="button" class="button`)
		out.ExtraClass(attrs)
		out.Static(`"`)
		out.PassAttrs(attrs, "type", "", "hx-*")
		out.Static("></button><p")
		out.Class("", attrs)
		out.Class("", nil)
		out.Static(">")
		return out.Err()
	}))
	want := `<button type="button" class="button wide dark" id="user" data-x="1"></button>` +
		`<p class="wide dark">`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if got := (Attrs{A("class", "a"), A("class", ""), A("class", "b")}).Class(); got != "a b" {
		t.Fatalf("Class() = %q", got)
	}
}

func TestUnsafeAttributeErrorIsSticky(t *testing.T) {
	var output strings.Builder
	err := Render(context.Background(), &output, El("div", Attrs{A("onclick", "x")}, Text("body")))
	if !errors.Is(err, ErrUnsafeAttribute) {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(output.String(), "onclick") || strings.Contains(output.String(), "body") {
		t.Fatalf("unsafe output leaked: %q", output.String())
	}
}

func BenchmarkElementWithAttrs(b *testing.B) {
	element := El("div", Attrs{A("class", "row"), A("data-id", "42"), A("hx-get", "/rows/42")},
		El("span", nil, Text("Ada")),
	)
	ctx := context.Background()
	b.ReportAllocs()
	for range b.N {
		if err := Render(ctx, io.Discard, element); err != nil {
			b.Fatal(err)
		}
	}
}
