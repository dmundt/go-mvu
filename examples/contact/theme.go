package main

import (
	_ "embed"

	"github.com/dmundt/go-mvu/theme"
)

//go:embed contact.css
var contactCSS string

type contactTheme struct{}

func (contactTheme) Name() string { return theme.DefaultTheme{}.Name() }

func (contactTheme) Stylesheet() string {
	return theme.DefaultTheme{}.Stylesheet() + "\n" + contactCSS
}
