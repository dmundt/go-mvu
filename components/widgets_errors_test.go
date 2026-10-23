package components

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

type widgetErrorWriter struct{ err error }

func (writer widgetErrorWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestWidgetsPropagateWriterFailures(t *testing.T) {
	failure := errors.New("widget writer failed")
	for _, widget := range []html.Component{
		Accordion{ID: "faq", Items: []AccordionItem{{Title: "Question", Body: Text("Answer")}}},
		Tabs{ID: "tabs", Label: "Sections", Active: "one",
			Items: []Tab{{ID: "one", Label: "One", URL: "/one"}}},
		Dialog{ID: "dialog", Title: "Details", TriggerLabel: "Open", Body: Text("Body")},
		Dropdown{Label: "Menu", Items: []MenuItem{{Label: "Home", URL: "/"}}},
		Tooltip{ID: "tip", Label: "Info", Content: "More"},
		Avatar{Name: "Ada"},
		Progress{ID: "progress", Label: "Upload", Max: 10, Value: 1},
		Spinner{}, Skeleton{}, Separator{}, Kbd{Keys: []string{"K"}},
		Stat{Label: "Members", Value: "18"},
		Steps{Label: "Steps", Items: []string{"One"}, Current: 0},
		Switch{Name: "alerts", Label: "Alerts"},
		RadioGroup{Name: "role", Legend: "Role", Options: []Option{{Value: "admin", Label: "Admin"}}},
	} {
		if err := html.Render(context.Background(), widgetErrorWriter{failure}, widget); !errors.Is(err, failure) {
			t.Errorf("%T: error = %v", widget, err)
		}
	}
}

func TestWidgetValidationUsesComponentsPrefix(t *testing.T) {
	for _, widget := range []html.Component{
		Dialog{Title: "Missing ID", TriggerLabel: "Open"},
		Avatar{Name: "Ada", Src: "javascript:alert(1)"},
		Switch{Name: "invalid id", Label: "Alerts"},
		RadioGroup{Name: "role"},
	} {
		var output strings.Builder
		err := html.Render(context.Background(), &output, widget)
		if err == nil || !strings.HasPrefix(err.Error(), "components:") || output.Len() != 0 {
			t.Errorf("%T: error=%v output=%q", widget, err, output.String())
		}
	}
}
