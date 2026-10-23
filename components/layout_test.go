package components

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func TestLayoutConstructorsPreserveComponents(t *testing.T) {
	body := Column(Row(Text("<first>"), Paragraph("second")))
	want := `<div class="stack"><div class="inline">&lt;first&gt;<p>second</p></div></div>`
	if got := render(t, body); got != want {
		t.Fatalf("layout = %q, want %q", got, want)
	}
	body.Gap = "0.5rem"
	body.Attrs = html.Attrs{html.A("aria-label", "Items")}
	if got := render(t, body); !strings.Contains(got, `stack--gap-sm" aria-label="Items"`) {
		t.Fatalf("custom layout = %s", got)
	}
	if got := render(t, FormLayout(Text("field"))); got != `<div class="stack form-fields">field</div>` {
		t.Fatalf("form layout = %s", got)
	}
}

func TestLayoutConstructorsPropagateErrors(t *testing.T) {
	want := errors.New("child failed")
	child := html.ComponentFunc(func(context.Context, *html.Context) error { return want })
	if err := html.Render(context.Background(), io.Discard, Column(Row(child))); !errors.Is(err, want) {
		t.Fatalf("render error = %v, want %v", err, want)
	}
}
