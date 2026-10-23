package components

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func TestSectionComposesAndEscapes(t *testing.T) {
	got := render(t, Section{
		ID: `s"x`,
		Attrs: html.Attrs{
			html.A("id", "ignored"), html.A("class", "region"),
			html.A("aria-labelledby", "title"), html.A("data-row", "<7>"),
		},
		Children: []html.Component{
			Heading{Level: 2, ID: "title", Text: "A&B"},
			Stack{Children: []html.Component{Text("<body>")}},
		},
	})
	want := `<section id="s&#34;x" class="region" aria-labelledby="title" data-row="&lt;7&gt;">` +
		`<h2 id="title">A&amp;B</h2><div class="stack">&lt;body&gt;</div></section>`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if got := render(t, Section{Attrs: html.Attrs{html.A("id", "extra")}}); got !=
		`<section id="extra"></section>` {
		t.Fatalf("passthrough ID: %s", got)
	}
}

func TestSectionPropagatesErrors(t *testing.T) {
	childError := errors.New("child failed")
	for _, tc := range []struct {
		section Section
		want    error
	}{
		{Section{Attrs: html.Attrs{html.A("style", "color:red")}}, html.ErrUnsafeAttribute},
		{Section{Children: []html.Component{html.ComponentFunc(
			func(context.Context, *html.Context) error { return childError },
		)}}, childError},
	} {
		if err := html.Render(context.Background(), io.Discard, tc.section); !errors.Is(err, tc.want) {
			t.Fatalf("got %v, want %v", err, tc.want)
		}
	}
}
