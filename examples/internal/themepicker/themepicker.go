// Package themepicker selects bundled themes by name and renders a GET form
// for choosing theme and mode. Shared by gallery examples.
package themepicker

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/theme/bootstrap"
	"github.com/dmundt/go-mvu/theme/bulma"
	"github.com/dmundt/go-mvu/theme/pico"
)

// Names lists selectable themes in display order.
var Names = []string{"default", "bootstrap", "bulma", "pico"}

var themeOptions = []components.Option{
	{Value: "default", Label: "Default"},
	{Value: "bootstrap", Label: "Bootstrap"},
	{Value: "bulma", Label: "Bulma"},
	{Value: "pico", Label: "Pico"},
}

var modeOptions = []components.Option{
	{Value: "light", Label: "Light"},
	{Value: "dark", Label: "Dark"},
}

// Lookup returns the theme for name. Empty name selects the default theme.
func Lookup(name string) (theme.Theme, error) {
	switch name {
	case "", "default":
		return theme.DefaultTheme{}, nil
	case "bootstrap":
		return bootstrap.Theme{}, nil
	case "bulma":
		return bulma.Theme{}, nil
	case "pico":
		return pico.Theme{}, nil
	default:
		return nil, fmt.Errorf("unknown theme %q", name)
	}
}

// Hidden is a query parameter preserved across theme changes.
type Hidden struct{ Name, Value string }

// Form renders theme and mode selects plus a submit button.
type Form struct {
	Theme  string
	Mode   theme.Mode
	Hidden []Hidden
}

func (form Form) RenderHTML(ctx context.Context, out *html.Context) error {
	children := []html.Component{
		components.Select{
			ID:      "theme",
			Name:    "theme",
			Label:   "CSS framework",
			Value:   form.Theme,
			Options: themeOptions,
		},
		components.Select{
			ID:      "mode",
			Name:    "mode",
			Label:   "Mode",
			Value:   form.Mode.String(),
			Options: modeOptions,
		},
	}
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
