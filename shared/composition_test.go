package shared

import (
	"context"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
)

func TestWidgetTriggersUseCoreButton(t *testing.T) {
	trigger := components.Button{
		Label: `Open <details>`, Variant: components.ButtonGhost,
		Attrs: html.Attrs{html.A("popovertarget", "dialog")},
	}
	got := render(t, Dialog{
		ID: "dialog", Title: "Details", TriggerLabel: trigger.Label,
		TriggerVariant: trigger.Variant,
	})
	if want := render(t, trigger); !strings.HasPrefix(got, want) {
		t.Fatalf("dialog trigger differs from core Button: %s, want prefix %s", got, want)
	}
	tooltipButton := render(t, components.Button{
		Label: "Info", Variant: components.ButtonSecondary,
		Attrs: html.Attrs{
			html.A("aria-describedby", "info"),
			html.A("aria-controls", "info"),
			html.A("popovertarget", "info"),
		},
	})
	if got := render(t, Tooltip{ID: "info", Label: "Info", Content: "More"}); !strings.Contains(got, tooltipButton) {
		t.Fatalf("tooltip trigger differs from core Button: %s", got)
	}
}

func TestInvalidDialogTriggerFailsBeforeOutput(t *testing.T) {
	var output strings.Builder
	err := html.Render(context.Background(), &output, Dialog{
		ID: "dialog", Title: "Details", TriggerLabel: "Open", TriggerVariant: "invalid",
	})
	if err == nil || output.Len() != 0 {
		t.Fatalf("invalid trigger: err=%v output=%s", err, output.String())
	}
}
