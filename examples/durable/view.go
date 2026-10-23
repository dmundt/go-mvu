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

type CounterFragment struct {
	html.FragmentMarker
	Count   int
	Pending int
}

func (counter CounterFragment) RenderHTML(ctx context.Context, out *html.Context) error {
	form := components.Form{
		URL:       "/increment",
		CSRFField: middleware.CSRFInput{},
		Children: []html.Component{components.Button{
			Type:  components.ButtonSubmit,
			Label: "Increment",
			Action: htmx.Action{
				Method: htmx.Post, URL: "/increment", Target: "#counter",
				Swap: htmx.OuterHTML,
			},
		}},
	}
	out.Component(ctx, components.Section{
		ID: "counter",
		Children: []html.Component{components.Stack{Children: []html.Component{
			components.Output{Value: strconv.Itoa(counter.Count)},
			components.Paragraph("Pending durable deliveries: " + strconv.Itoa(counter.Pending)),
			form,
		}}},
	})
	return out.Err()
}

type CounterView struct{}

func (CounterView) Page(
	_ context.Context,
	model CounterModel,
	request web.RequestContext,
) (html.Page, error) {
	page := components.Shell{
		Header: components.Heading{Level: 1, Text: "Durable counter and outbox"},
		Main: components.Card{
			Header: components.Heading{Level: 2, Text: "Counter"},
			Body: CounterFragment{
				Count: model.Count, Pending: len(model.Outbox),
			},
		},
		Footer: components.Paragraph(
			"State and pending events survive process restarts. Delivery is at least once; " +
				"event IDs make this sink idempotent.",
		),
	}
	return components.Document{
		Title:         "Durable counter",
		HTMXSource:    static.HTMXPath,
		HTMXIntegrity: static.HTMXIntegrity,
		Theme:         request.Theme,
		Mode:          request.Mode,
		Body:          middleware.CSRFHTMX{Body: page},
	}, nil
}

func (CounterView) Fragment(
	_ context.Context,
	name string,
	model CounterModel,
	_ web.RequestContext,
) (html.Fragment, error) {
	if name != "counter" {
		return nil, web.ErrUnknownFragment
	}
	return CounterFragment{Count: model.Count, Pending: len(model.Outbox)}, nil
}
