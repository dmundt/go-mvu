package modepicker

import (
	"context"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
)

func TestFormSelectsCurrentModeAndEscapesHidden(t *testing.T) {
	var out strings.Builder
	form := Form{
		Mode:   theme.Dark,
		Hidden: []Hidden{{Name: "tab", Value: `a"b`}},
	}
	if err := html.Render(context.Background(), &out, form); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<form method="get" action="?">`,
		`<option value="dark" selected>`,
		`<input class="input" id="tab" name="tab" type="hidden" value="a&#34;b">`,
		`type="submit"`,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("form missing %q in %s", want, out.String())
		}
	}
}
