package main

import (
	"context"
	"strconv"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/web"
)

const counterRegion ui.Region = "counter"

func counterControls(count int) html.Component {
	return components.Column(components.Row(
		ui.ActionButton{
			Label:     "-",
			AriaLabel: "Decrement",
			Action:    counterDecrement.HTMX(),
		},
		components.Output{Value: strconv.Itoa(count), Live: true},
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
	))
}

type CounterView struct{ Document ui.Document }

func (CounterView) Fragment(
	_ context.Context, name string, model CounterModel, _ web.RequestContext,
) (html.Fragment, error) {
	return counterRegion.Select(name, counterControls(model.Count))
}

func (view CounterView) Page(
	_ context.Context,
	model CounterModel,
	request web.RequestContext,
) (html.Page, error) {
	shell := components.AppShell{
		Header: components.Row(
			components.Heading{Level: 1, Text: "Go MVU counter"},
			ui.ThemeSwitcher{
				Mode:   request.Mode,
				Return: request.CurrentURL,
			},
		),
		Sidebar: components.Navigation{Label: "Main", Items: []components.NavItem{
			{Label: "Counter", URL: "/", Current: true},
		}},
		Main: components.Card{
			Header: components.Heading{Level: 2, Text: "Count"},
			Body:   counterRegion.Wrap(counterControls(model.Count)),
		},
		Footer: components.Paragraph("Server-rendered MVU. No application JavaScript."),
	}
	return view.Document.Page(request, "Go MVU Counter", shell), nil
}
