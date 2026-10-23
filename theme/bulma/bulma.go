// Package bulma adapts Bulma CSS to go-mvu components without browser JavaScript.
package bulma

import (
	_ "embed"

	"github.com/dmundt/go-mvu/theme"
)

//go:embed framework.css
var frameworkCSS string

var stylesheet = theme.DefaultTheme{}.Stylesheet() + "\n" + frameworkCSS

type Theme struct{}

func (Theme) Name() string       { return "bulma" }
func (Theme) Stylesheet() string { return stylesheet }
