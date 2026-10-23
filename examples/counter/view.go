package main

import (
	"context"
	"strconv"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/static"
	"github.com/dmundt/go-mvu/web"
)

// CounterFragment is the HTMX swap target. Buttons work without
// JavaScript too: each sits in a POST form that redirects back.
type CounterFragment struct {
	html.FragmentMarker
	Count int
}

func (counter CounterFragment) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, components.Section{
		ID: "counter",
		Children: []html.Component{components.Stack{
			Children: []html.Component{CounterControls{Count: counter.Count}},
		}},
	})
	return out.Err()
}

// CounterControls groups the current value and its no-JavaScript-capable actions.
type CounterControls struct{ Count int }

func (controls CounterControls) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, components.Inline{Children: []html.Component{
		counterButton("-", "Decrement", "/counter/decrement", ""),
		components.Output{Value: strconv.Itoa(controls.Count), Live: true},
		counterButton("+", "Increment", "/counter/increment", ""),
		counterButton("Reset", "", "/counter/reset", components.ButtonSecondary),
	}})
	return out.Err()
}

// counterButton posts to url and swaps the whole counter section.
func counterButton(
	label, ariaLabel, url string,
	variant components.ButtonVariant,
) html.Component {
	button := components.Button{
		Type:      components.ButtonSubmit,
		Label:     label,
		AriaLabel: ariaLabel,
		Variant:   variant,
		Action: htmx.Action{
			Method: htmx.Post,
			URL:    url,
			Target: "#counter",
			Swap:   htmx.OuterHTML,
		},
	}
	return components.Form{
		URL:       url,
		CSRFField: middleware.CSRFInput{},
		Children:  []html.Component{button},
	}
}

type CounterView struct{}

func (CounterView) Page(
	_ context.Context,
	model CounterModel,
	request web.RequestContext,
) (html.Page, error) {
	shell := components.AppShell{
		Header: components.Inline{Children: []html.Component{
			components.Heading{Level: 1, Text: "Go MVU counter"},
			components.ThemeSwitcher{
				Mode:      request.Mode,
				Return:    request.CurrentURL,
				CSRFField: middleware.CSRFInput{},
			},
		}},
		Sidebar: components.Navigation{Label: "Main", Items: []components.NavItem{
			{Label: "Counter", URL: "/", Current: true},
		}},
		Main: components.Card{
			Header: components.Heading{Level: 2, Text: "Count"},
			Body:   CounterFragment{Count: model.Count},
		},
		Footer: components.Paragraph("Server-rendered MVU. No application JavaScript."),
	}
	return components.Document{
		Title:                    "Go MVU Counter",
		HTMXSource:               static.HTMXPath,
		HTMXIntegrity:            static.HTMXIntegrity,
		HTMXSwapValidationErrors: true,
		Theme:                    request.Theme,
		Mode:                     request.Mode,
		Body:                     middleware.CSRFHTMX{Body: shell},
	}, nil
}

func (CounterView) Fragment(
	_ context.Context,
	name string,
	model CounterModel,
	_ web.RequestContext,
) (html.Fragment, error) {
	switch name {
	case "counter":
		return CounterFragment{Count: model.Count}, nil
	default:
		return nil, web.ErrUnknownFragment
	}
}
