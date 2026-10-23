// Package modepicker renders a GET form for choosing theme mode.
package modepicker

import (
	"context"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
)

var modeOptions = []components.Option{
	{Value: "system", Label: "System"},
	{Value: "light", Label: "Light"},
	{Value: "dark", Label: "Dark"},
}

// Hidden is a query parameter preserved across mode changes.
type Hidden struct{ Name, Value string }

// Form renders a mode select and submit button.
type Form struct {
	Mode   theme.Mode
	Hidden []Hidden
}

func (form Form) RenderHTML(ctx context.Context, out *html.Context) error {
	children := []html.Component{components.Select{
		ID:      "mode",
		Name:    "mode",
		Label:   "Mode",
		Value:   form.Mode.String(),
		Options: modeOptions,
	}}
	for _, field := range form.Hidden {
		children = append(children, components.Input{
			Type:  "hidden",
			Name:  field.Name,
			Value: field.Value,
		})
	}
	children = append(children, components.Button{
		Type:  components.ButtonSubmit,
		Label: "Apply theme",
	})
	out.Component(ctx, components.Form{
		Method:   components.FormGet,
		URL:      "?",
		Children: children,
	})
	return out.Err()
}
