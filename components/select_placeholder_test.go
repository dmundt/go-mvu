package components

import (
	"context"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func TestSelectPlaceholderEscapingAndSelection(t *testing.T) {
	for _, value := range []string{"", "support"} {
		selectInput := Select{
			Name: "topic", Label: "Topic", Value: value, Required: true,
			Placeholder: "<Choose>", Options: []Option{{Value: "support", Label: "Support"}},
		}
		output := render(t, selectInput)
		placeholder := `<option value="">`
		if value == "" {
			placeholder = `<option value="" selected>`
		}
		if !strings.Contains(output, placeholder+`&lt;Choose&gt;</option>`) ||
			strings.Count(output, " selected") != 1 {
			t.Fatalf("value %q: %s", value, output)
		}
		if value != "" && !strings.Contains(output, `<option value="support" selected>Support</option>`) {
			t.Fatalf("lost selected value: %s", output)
		}
		if strings.Contains(output, " disabled") {
			t.Fatal("placeholder must remain selectable")
		}
	}
}

func TestSelectPlaceholderRejectsDuplicateEmptyOptionBeforeOutput(t *testing.T) {
	var output strings.Builder
	err := html.Render(context.Background(), &output, Select{
		Name: "topic", Placeholder: "Choose", Options: []Option{{Label: "Other placeholder"}},
	})
	if err == nil || output.Len() != 0 {
		t.Fatalf("error = %v, output = %q", err, output.String())
	}
}
