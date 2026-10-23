package themepicker

import (
	"context"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
)

func TestLookup(t *testing.T) {
	for _, name := range append([]string{""}, Names...) {
		selected, err := Lookup(name)
		if err != nil {
			t.Fatalf("Lookup(%q): %v", name, err)
		}
		want := name
		if want == "" {
			want = "default"
		}
		if selected.Name() != want {
			t.Errorf("Lookup(%q).Name() = %q", name, selected.Name())
		}
	}
	if _, err := Lookup("tailwind"); err == nil {
		t.Error("Lookup accepted unknown theme")
	}
}

func TestFormSelectsCurrentValuesAndEscapesHidden(t *testing.T) {
	var out strings.Builder
	form := Form{
		Theme:  "bulma",
		Mode:   theme.Dark,
		Hidden: []Hidden{{Name: "tab", Value: `a"b`}},
	}
	if err := html.Render(context.Background(), &out, form); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<form method="get">`,
		`<option value="bulma" selected>`,
		`<option value="dark" selected>`,
		`<input type="hidden" name="tab" value="a&#34;b">`,
		`type="submit"`,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("form missing %q in %s", want, out.String())
		}
	}
}
