package components

import (
	"context"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

func render(t *testing.T, component html.Component) string {
	t.Helper()
	var output strings.Builder
	if err := html.Render(context.Background(), &output, component); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestPrimitivesRenderEscapedMarkup(t *testing.T) {
	cases := []struct {
		name      string
		component html.Component
		want      string
	}{
		{"paragraph", Paragraph("a<b"), "<p>a&lt;b</p>"},
		{"heading", Heading{Level: 2, Text: "x&y"}, "<h2>x&amp;y</h2>"},
		{"heading id", Heading{Level: 1, ID: `t"1`, Text: "T"}, `<h1 id="t&#34;1">T</h1>`},
		{"output", Output{Value: "<5>"}, "<output>&lt;5&gt;</output>"},
		{
			"live output",
			Output{ID: "n", Value: "5", Live: true},
			`<output id="n" aria-live="polite">5</output>`,
		},
		{
			"button",
			Button{ID: "inc", Label: "+", AriaLabel: "Increment"},
			`<button id="inc" type="button" class="button" aria-label="Increment">+</button>`,
		},
		{
			"button link",
			Link{Label: "Go", URL: "/a", Variant: ButtonSecondary},
			`<a href="/a" class="button button--secondary">Go</a>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := render(t, tc.component); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestButtonRendersHTMXAction(t *testing.T) {
	got := render(t, Button{
		Label:  "Save",
		Action: web.BrowserAction{Method: web.Post, URL: "/save", Target: "#form"},
	})
	for _, want := range []string{`hx-post="/save"`, `hx-target="#form"`} {
		if !strings.Contains(got, want) {
			t.Errorf("button missing %q: %s", want, got)
		}
	}
}

func TestPrimitivesRejectInvalidInput(t *testing.T) {
	for name, component := range map[string]html.Component{
		"heading level 0":  Heading{Level: 0, Text: "x"},
		"heading level 7":  Heading{Level: 7, Text: "x"},
		"button variant":   Button{Label: "x", Variant: "fancy"},
		"link variant":     Link{Label: "x", URL: "/", Variant: "fancy"},
		"button htmx type": Button{Label: "x", Type: "image"},
	} {
		t.Run(name, func(t *testing.T) {
			var output strings.Builder
			if err := html.Render(context.Background(), &output, component); err == nil {
				t.Errorf("render accepted invalid input: %s", output.String())
			}
		})
	}
}
