package components

import (
	"context"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
)

var modeOptions = []Option{
	{Value: "system", Label: "System"},
	{Value: "light", Label: "Light"},
	{Value: "dark", Label: "Dark"},
}

// ModePickerField is a query parameter preserved across mode changes.
type ModePickerField struct{ Name, Value string }

// ModePicker submits a GET form for previewing a theme mode. The application
// reads the mode query parameter; no preference cookie or CSRF token is needed.
type ModePicker struct {
	Mode   theme.Mode
	Hidden []ModePickerField
}

func (form ModePicker) RenderHTML(ctx context.Context, out *html.Context) error {
	children := []html.Component{Select{
		ID:      "mode",
		Name:    "mode",
		Label:   "Mode",
		Value:   form.Mode.String(),
		Options: modeOptions,
	}}
	for _, field := range form.Hidden {
		children = append(children, Input{
			Type:  "hidden",
			Name:  field.Name,
			Value: field.Value,
		})
	}
	children = append(children, Button{
		Type:  ButtonSubmit,
		Label: "Apply theme",
	})
	out.Component(ctx, Form{
		Method:   FormGet,
		URL:      "?",
		Children: children,
	})
	return out.Err()
}
