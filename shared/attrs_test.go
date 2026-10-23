package shared

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func TestSharedAttrsPassthrough(t *testing.T) {
	cases := []struct {
		name      string
		component html.Component
		want      string
	}{
		{"progress polls", Progress{ID: "p", Label: "Upload", Value: 1, Max: 2, Attrs: html.Attrs{
			html.A("hx-get", "/progress"), html.A("hx-trigger", "every 2s"), html.A("class", "compact"),
		}}, `<div class="progress-field compact" hx-get="/progress" hx-trigger="every 2s">`},
		{"spinner keeps role", Spinner{ID: "s", Attrs: html.Attrs{html.A("role", "x"), html.A("data-k", "v")}},
			`<span class="spinner" role="status" id="s" data-k="v">`},
		{"dialog panel", Dialog{ID: "d", Title: "T", TriggerLabel: "Open", Attrs: html.Attrs{
			html.A("class", "wide"), html.A("popover", "manual"),
		}}, `<div class="dialog wide" popover role="dialog" id="d" aria-labelledby="d-title">`},
		{"switch input", Switch{Name: "n", Label: "L", Attrs: html.Attrs{html.A("hx-post", "/toggle")}},
			`role="switch" id="n" name="n" value="on" hx-post="/toggle">`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := render(t, testCase.component); !strings.Contains(got, testCase.want) {
				t.Fatalf("got  %s\nwant %s", got, testCase.want)
			}
		})
	}
	err := html.Render(context.Background(), io.Discard,
		Kbd{Keys: []string{"K"}, Attrs: html.Attrs{html.A("onmouseover", "x")}})
	if !errors.Is(err, html.ErrUnsafeAttribute) {
		t.Fatalf("got %v, want ErrUnsafeAttribute", err)
	}
}
