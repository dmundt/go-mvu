package main

import (
	"context"
	"strconv"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/web"
)

const counterRegion ui.Region = "counter"

// CounterControls groups the current value and its no-JavaScript-capable actions.
type CounterControls struct{ Count int }

func (controls CounterControls) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, components.Column(components.Row(
		ui.ActionButton{
			Label:     "-",
			AriaLabel: "Decrement",
			Action:    counterDecrement.HTMX(),
		},
		components.Output{Value: strconv.Itoa(controls.Count), Live: true},
		ui.ActionButton{
			Label:     "+",
			AriaLabel: "Increment",
			Action:    counterIncrement.HTMX(),
		},
		ui.ActionButton{
			Label:   "Reset",
			Action:  counterReset.HTMX(),
			Variant: components.ButtonSecondary,
		},
	)))
	return out.Err()
}

type CounterView struct{ Document ui.Document }

func (view CounterView) Page(
	_ context.Context,
	model CounterModel,
	request web.RequestContext,
) (html.Page, error) {
	shell := components.AppShell{
		Header: components.Row(
			components.Heading{Level: 1, Text: "Go MVU counter"},
			components.ThemeSwitcher{
				Mode:      request.Mode,
				Return:    request.CurrentURL,
				CSRFField: middleware.CSRFInput{},
			},
		),
		Sidebar: components.Navigation{Label: "Main", Items: []components.NavItem{
			{Label: "Counter", URL: "/", Current: true},
		}},
		Main: components.Card{
			Header: components.Heading{Level: 2, Text: "Count"},
			Body:   counterRegion.Wrap(CounterControls{Count: model.Count}),
		},
		Footer: components.Paragraph("Server-rendered MVU. No application JavaScript."),
	}
	return view.Document.Page(request, "Go MVU Counter", shell), nil
}

func (CounterView) Fragment(
	_ context.Context,
	name string,
	model CounterModel,
	_ web.RequestContext,
) (html.Fragment, error) {
	return counterRegion.Select(name, CounterControls{Count: model.Count})
}
