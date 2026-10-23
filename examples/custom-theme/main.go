// Command custom-theme demonstrates mapping application CSS tokens to go-mvu
// component classes through theme.Theme.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

func main() {
	log.Println("custom theme example listening on http://localhost:8085")
	log.Fatal(http.ListenAndServe("localhost:8085", newHandler()))
}

func newHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := components.Document{
			Title: "Custom CSS framework adapter",
			Theme: orbitTheme{},
			Mode:  theme.Dark,
			Body:  orbitGallery{},
		}
		if err := web.WritePage(r.Context(), w, http.StatusOK, page); err != nil {
			log.Printf("render custom theme example: %v", err)
		}
	})
}

type orbitGallery struct{}

func (orbitGallery) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, components.Main{Attrs: html.Attrs{html.A("class", "orbit-main")}, Children: []html.Component{
		components.Heading{Level: 1, Text: "Orbit CSS theme"},
		components.Paragraph("App-owned Orbit tokens style go-mvu semantic components."),
		components.Card{
			Variant: components.CardPrimary,
			Header:  components.Heading{Level: 2, Text: "Mapped component classes"},
			Body: components.Stack{Children: []html.Component{
				components.Badge{Kind: "success", Label: "Theme adapter active"},
				components.Input{Name: "email", Label: "Email", Value: "ada@example.test"},
				components.Sequence{
					components.Button{Label: "Primary", Variant: components.ButtonPrimary},
					components.Button{Label: "Secondary", Variant: components.ButtonSecondary},
				},
			}},
		},
	}})
	return out.Err()
}
