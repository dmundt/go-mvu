package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
)

func TestModePickerSelectsCurrentModeAndEscapesHidden(t *testing.T) {
	var out strings.Builder
	form := ModePicker{
		Mode:   theme.Dark,
		Hidden: []ModePickerField{{Name: "tab", Value: `a"b`}},
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

func TestModePickerOffersAllModesWithoutCSRF(t *testing.T) {
	for _, mode := range []theme.Mode{theme.System, theme.Light, theme.Dark} {
		output := render(t, ModePicker{Mode: mode})
		for _, value := range []string{"system", "light", "dark"} {
			want := `<option value="` + value + `"`
			if value == mode.String() {
				want += ` selected`
			}
			if !strings.Contains(output, want+`>`) {
				t.Errorf("mode=%s missing %q: %s", mode.String(), want, output)
			}
		}
		if strings.Contains(output, "_csrf") || strings.Contains(output, "hx-post") {
			t.Fatalf("preview form must not mutate preferences: %s", output)
		}
	}
}

func TestModePickerDefaultsToSystem(t *testing.T) {
	output := render(t, ModePicker{})
	if !strings.Contains(output, `<option value="system" selected>`) {
		t.Fatalf("zero-value picker must select system: %s", output)
	}
}

func TestModePickerEscapesHiddenNamesAndValues(t *testing.T) {
	output := render(t, ModePicker{
		Hidden: []ModePickerField{{Name: `a"b`, Value: `<tag>&"`}},
	})
	for _, want := range []string{`name="a&#34;b"`, `value="&lt;tag&gt;&amp;&#34;"`} {
		if !strings.Contains(output, want) {
			t.Errorf("picker missing escaped field %q: %s", want, output)
		}
	}
}

func TestModePickerPropagatesWriterErrors(t *testing.T) {
	failure := errors.New("mode picker writer failed")
	if err := html.Render(context.Background(), failedWriter{err: failure},
		ModePicker{Mode: theme.Light}); !errors.Is(err, failure) {
		t.Fatalf("render error = %v, want %v", err, failure)
	}
}
