// Package theme provides server-controlled visual themes and light, dark, and
// system color modes. System mode follows the browser through CSS
// prefers-color-scheme; no JavaScript runs.
package theme

import (
	_ "embed"
	"errors"
	"strings"
)

type Mode uint8

const (
	// System follows the operating system via @media (prefers-color-scheme).
	System Mode = iota
	Light
	Dark
)

var ErrInvalidMode = errors.New("invalid theme mode")

// Modes lists every mode in display order.
var Modes = [...]Mode{System, Light, Dark}

func (m Mode) String() string {
	switch m {
	case Light:
		return "light"
	case Dark:
		return "dark"
	default:
		return "system"
	}
}

// Label is the human-readable button label for m.
func (m Mode) Label() string {
	switch m {
	case Light:
		return "Light"
	case Dark:
		return "Dark"
	default:
		return "System"
	}
}

// ParseMode accepts exactly "system", "light", or "dark".
func ParseMode(value string) (Mode, error) {
	switch value {
	case "system":
		return System, nil
	case "light":
		return Light, nil
	case "dark":
		return Dark, nil
	default:
		return System, ErrInvalidMode
	}
}

// Parse is lenient: it trims, ignores case, and returns System for unknown
// values. Use ParseMode to reject invalid input.
func Parse(value string) Mode {
	mode, _ := ParseMode(strings.ToLower(strings.TrimSpace(value)))
	return mode
}

// Toggle switches between light and dark. System toggles to dark.
func (m Mode) Toggle() Mode {
	if m == Dark {
		return Light
	}
	return Dark
}

// Theme supplies an inlined stylesheet. Components emit semantic classes;
// the stylesheet maps them to tokens, so no per-component lookup runs.
type Theme interface {
	Name() string
	Stylesheet() string
}

// DocumentClasser is optionally implemented by a Theme that needs a class on
// the root html element for mode-specific styling. Document
// calls it once per page; components never perform theme lookups.
type DocumentClasser interface {
	DocumentClass(Mode) string
}

// DocumentClass returns t's root class for mode, or "" when t does not
// implement DocumentClasser.
func DocumentClass(t Theme, mode Mode) string {
	if classer, ok := t.(DocumentClasser); ok {
		return classer.DocumentClass(mode)
	}
	return ""
}

type DefaultTheme struct{}

func (DefaultTheme) Name() string { return "default" }

// Default design system assets live in theme/default/:
//   - open-props.css: generated Open Props token subset
//   - base.css: neutral palette, public tokens, structure
//   - default.css: design refinements scoped to data-theme-name="default"
//   - shared.css: optional shared package widgets
var (
	//go:embed default/open-props.css
	openPropsCSS string
	//go:embed default/base.css
	baseCSS string
	//go:embed default/default.css
	defaultCSS string
	//go:embed default/shared.css
	sharedCSS string
)

var defaultStylesheet = strings.Join(
	[]string{openPropsCSS, baseCSS, defaultCSS, sharedCSS}, "\n",
)

func (DefaultTheme) Stylesheet() string { return defaultStylesheet }

var Default Theme = DefaultTheme{}
