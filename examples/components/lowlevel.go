package main

import (
	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
)

// Explicit escape-hatch demo: generic HTML and semantic components share
// html.Component. Keep this interoperability example out of ordinary views.
func mixedCard() html.Component {
	release := html.El("p", nil,
		html.Text("Released "),
		html.El("time", html.Attrs{html.A("datetime", "2025-01-15")}, html.Text("Jan 15")),
		html.Text(" after "),
		html.El("abbr", html.Attrs{html.A("title", "Continuous integration")}, html.Text("CI")),
		html.Text(" passed."),
	)
	badge := components.Badge{
		Kind:  "success",
		Label: "Stable",
		Attrs: html.Attrs{html.A("title", "No open incidents"), html.A("data-channel", "stable")},
	}
	return card("Escape hatch: mixing levels", release, inline(badge, components.Button{
		Label:   "Release notes",
		Variant: components.ButtonGhost,
		Attrs:   html.Attrs{html.A("aria-describedby", "save-tip")},
	}))
}
