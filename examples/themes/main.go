// Command themes serves one set of framework components in every bundled theme.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/examples/internal/themepicker"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

func main() {
	log.Println("theme gallery listening on http://localhost:8082")
	log.Fatal(http.ListenAndServe("localhost:8082", newHandler()))
}

func newHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		selected, err := themepicker.Lookup(query.Get("theme"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mode := theme.Parse(query.Get("mode"))
		document := components.Document{
			Title: "Theme gallery",
			Theme: selected,
			Mode:  mode,
			Body:  gallery{Theme: selected.Name(), Mode: mode},
		}
		if err := web.WritePage(r.Context(), w, http.StatusOK, document); err != nil {
			log.Printf("render theme gallery: %v", err)
		}
	})
}

type gallery struct {
	Theme string
	Mode  theme.Mode
}

func (view gallery) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Static(`<main>`)
	out.Component(ctx, components.Heading{Level: 1, Text: "Theme gallery"})
	out.Component(ctx, themepicker.Form{Theme: view.Theme, Mode: view.Mode})
	out.Component(ctx, components.Stack{Children: []html.Component{
		buttonCard(),
		components.Alert{
			Kind:    components.AlertSuccess,
			Title:   "Ready",
			Message: "No markup changes needed.",
		},
		components.Badge{Kind: "info", Label: "Information"},
		components.Input{ID: "sample-input", Name: "name", Label: "Name", Value: "Ada"},
		usersTable,
	}})
	out.Static(`</main>`)
	return out.Err()
}

func buttonCard() html.Component {
	return components.Card{
		ID:     "sample-card",
		Header: components.Text("Same components, different theme"),
		Body: components.Sequence{
			components.Button{Label: "Primary", Variant: components.ButtonPrimary},
			components.Button{Label: "Secondary", Variant: components.ButtonSecondary},
			components.Button{Label: "Danger", Variant: components.ButtonDanger},
			components.Button{Label: "Ghost", Variant: components.ButtonGhost},
		},
	}
}

// usersTable is plain semantic markup; themes style it without classes.
var usersTable = html.ComponentFunc(func(_ context.Context, out *html.Context) error {
	out.Static(`<table><caption>Users</caption>`)
	out.Static(`<thead><tr><th scope="col">Name</th><th scope="col">Email</th></tr></thead>`)
	out.Static(`<tbody>`)
	out.Static(`<tr><td>Ada Lovelace</td><td>ada@example.test</td></tr>`)
	out.Static(`<tr><td>Grace Hopper</td><td>grace@example.test</td></tr>`)
	out.Static(`</tbody></table>`)
	return out.Err()
})
