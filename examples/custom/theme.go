package main

import _ "embed"

//go:embed orbit.css
var orbitCSS string

type orbitTheme struct{}

func (orbitTheme) Name() string       { return "orbit" }
func (orbitTheme) Stylesheet() string { return orbitCSS }
