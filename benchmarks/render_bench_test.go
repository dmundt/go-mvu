package benchmarks

import (
	"context"
	"fmt"
	htmltemplate "html/template"
	"io"
	"strconv"
	"testing"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

const hostile = `<script>alert("x")</script> & 'quotes' plus ordinary text`

type staticFragment struct{ html.FragmentMarker }

func (staticFragment) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static(`<section class="card"><h2>Static</h2><p>Constant markup only.</p></section>`)
	return out.Err()
}

type textFragment struct {
	html.FragmentMarker
	Value string
}

func (fragment textFragment) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<p>")
	out.Text(fragment.Value)
	out.Static("</p>")
	return out.Err()
}

// counterFragment mirrors docs/engineering-brief.md §11.
type counterFragment struct {
	html.FragmentMarker
	Count int
}

var (
	decrement = web.BrowserAction{Method: web.Post, URL: "/counter/decrement", Target: "#counter", Swap: web.OuterHTML}
	increment = web.BrowserAction{Method: web.Post, URL: "/counter/increment", Target: "#counter", Swap: web.OuterHTML}
)

func (counter counterFragment) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static(`<div id="counter"><button type="button"`)
	if err := decrement.RenderAttributes(out); err != nil {
		return err
	}
	out.Static(`>-</button><output>`)
	out.Int(counter.Count)
	out.Static(`</output><button type="button"`)
	if err := increment.RenderAttributes(out); err != nil {
		return err
	}
	out.Static(`>+</button></div>`)
	return out.Err()
}

func dashboard(rows int) html.Component {
	body := make(components.Sequence, rows)
	for i := range body {
		body[i] = components.TableRow{Cells: []html.Component{
			components.TableCell{Header: true, Scope: components.ScopeRow, Body: components.Text("User " + strconv.Itoa(i))},
			components.TableCell{Body: components.Text(hostile)},
		}}
	}
	return components.Shell{
		Header:  components.Heading{Level: 1, Text: "Dashboard"},
		Sidebar: components.Navigation{Items: []components.NavItem{{Label: "Home", URL: "/", Current: true}, {Label: "Users", URL: "/users"}}},
		Main: components.Card{
			Header: components.Heading{Level: 2, Text: "Users"},
			Body: components.Table{
				Caption: "Users",
				Header:  components.HeaderRow{"Name", "Note"},
				Body:    body,
			},
		},
		Footer: components.Paragraph("Footer"),
	}
}

func run(b *testing.B, component html.Component) {
	b.Helper()
	ctx := context.Background()
	b.ReportAllocs()
	for range b.N {
		if err := html.Render(ctx, io.Discard, component); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStatic(b *testing.B) { run(b, staticFragment{}) }

func BenchmarkEscapedText(b *testing.B) { run(b, textFragment{Value: hostile}) }

func BenchmarkCounterFragment(b *testing.B) { run(b, counterFragment{Count: 42}) }

func BenchmarkDashboard(b *testing.B) {
	for _, rows := range []int{10, 100} {
		b.Run(fmt.Sprintf("rows=%d", rows), func(b *testing.B) { run(b, dashboard(rows)) })
	}
}

func BenchmarkPagination(b *testing.B) {
	for _, pages := range []int{10, 1000} {
		b.Run(fmt.Sprintf("pages=%d", pages), func(b *testing.B) {
			run(b, components.Pagination{
				Page: pages / 2, PageSize: 10, TotalItems: pages * 10,
				BaseURL: "/users?sort=name", Target: "#results", PushURL: true,
			})
		})
	}
}

var counterTemplate = htmltemplate.Must(htmltemplate.New("counter").Parse(
	`<div id="counter"><button type="button" hx-post="/counter/decrement" hx-target="#counter" hx-swap="outerHTML">-</button>` +
		`<output>{{.}}</output>` +
		`<button type="button" hx-post="/counter/increment" hx-target="#counter" hx-swap="outerHTML">+</button></div>`))

var textTemplate = htmltemplate.Must(htmltemplate.New("text").Parse(`<p>{{.}}</p>`))

// Baselines: html/template for the same output.
func BenchmarkTemplateCounter(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		if err := counterTemplate.Execute(io.Discard, 42); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTemplateEscapedText(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		if err := textTemplate.Execute(io.Discard, hostile); err != nil {
			b.Fatal(err)
		}
	}
}
