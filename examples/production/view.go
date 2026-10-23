package main

import (
	"context"
	"strconv"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/static"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

type Counter struct {
	html.FragmentMarker
	Count int
}

func (counter Counter) RenderHTML(ctx context.Context, out *html.Context) error {
	return html.El("section", html.Attrs{html.A("id", "counter")},
		components.Heading{Level: 2, Text: "Your durable counter"},
		components.Output{Value: strconv.Itoa(counter.Count), Live: true},
		components.Form{
			URL:       "/increment",
			CSRFField: middleware.CSRFInput{},
			Children: []html.Component{components.Button{
				Label: "Increment",
				Type:  components.ButtonSubmit,
				Action: htmx.Action{
					Method: htmx.Post,
					URL:    "/increment",
					Target: "#counter",
					Swap:   htmx.OuterHTML,
				},
			}},
		},
	).RenderHTML(ctx, out)
}

type View struct{}

func (View) Page(
	_ context.Context, model Model, request web.RequestContext,
) (html.Page, error) {
	return document("Durable Go MVU", components.Stack{Children: []html.Component{
		components.Heading{Level: 1, Text: "Durable Go MVU"},
		Counter{Count: model.Count},
		components.Form{
			URL:       "/logout",
			CSRFField: middleware.CSRFInput{},
			Children: []html.Component{components.Button{
				Label: "Log out",
				Type:  components.ButtonSubmit,
			}},
		},
	}}, request.Theme, request.Mode), nil
}

func (View) Fragment(
	_ context.Context, name string, model Model, _ web.RequestContext,
) (html.Fragment, error) {
	if name != "counter" {
		return nil, web.ErrUnknownFragment
	}
	return Counter{Count: model.Count}, nil
}

func document(
	title string, body html.Component, selected theme.Theme, mode theme.Mode,
) html.Page {
	return components.Document{
		Title:         title,
		HTMXSource:    static.HTMXPath,
		HTMXIntegrity: static.HTMXIntegrity,
		Theme:         selected,
		Mode:          mode,
		Body: middleware.CSRFHTMX{Body: components.AppShell{
			Main: components.Card{Body: body},
		}},
	}
}

func loginPage() html.Page {
	return document("Log in", components.Stack{Children: []html.Component{
		components.Heading{Level: 1, Text: "Go MVU production example"},
		components.Paragraph("Log in with your configured OpenID Connect provider."),
		components.Form{
			URL:       "/login",
			CSRFField: middleware.CSRFInput{},
			Children: []html.Component{components.Button{
				Label: "Log in",
				Type:  components.ButtonSubmit,
			}},
		},
	}}, theme.Default, theme.System)
}
