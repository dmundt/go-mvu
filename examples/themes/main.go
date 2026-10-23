// Command themes demonstrates the default theme in each color mode.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/web"
)

func main() {
	log.Println("theme gallery listening on http://localhost:8082")
	log.Fatal(http.ListenAndServe("localhost:8082", newHandler()))
}

func newHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		mode := theme.Parse(query.Get("mode"))
		document := components.Document{
			Title: "Default theme modes",
			Mode:  mode,
			Body:  gallery{Mode: mode},
		}
		if err := web.WritePage(r.Context(), w, http.StatusOK, document); err != nil {
			log.Printf("render theme gallery: %v", err)
		}
	})
}

type gallery struct {
	Mode theme.Mode
}

func (view gallery) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, components.Main{Children: []html.Component{
		components.Heading{Level: 1, Text: "Default theme modes"},
		ui.ModePicker{Mode: view.Mode},
		components.Stack{Children: []html.Component{
			buttonCard(),
			components.Alert{
				Kind:    components.AlertSuccess,
				Title:   "Ready",
				Message: "No markup changes needed.",
			},
			components.Badge{Kind: "info", Label: "Information"},
			components.Input{ID: "sample-input", Name: "name", Label: "Name", Value: "Ada"},
			usersTable,
		}},
	}})
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
