// Command components serves a gallery of the shared UI components in every
// bundled theme.
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/examples/internal/themepicker"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

func main() {
	log.Println("component gallery listening on http://localhost:8083")
	log.Fatal(http.ListenAndServe("localhost:8083", newHandler()))
}

func newHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		selected, err := themepicker.Lookup(query.Get("theme"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tab, err := lookupTab(query.Get("tab"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		view := gallery{
			Theme: selected.Name(),
			Mode:  theme.Parse(query.Get("mode")),
			Tab:   tab,
		}
		document := components.Document{
			Title: "Component gallery",
			Theme: selected,
			Mode:  view.Mode,
			Body:  view,
		}
		if err := web.WritePage(r.Context(), w, http.StatusOK, document); err != nil {
			log.Printf("render component gallery: %v", err)
		}
	})
}

// lookupTab returns the tab for id. Empty id selects the first tab.
func lookupTab(id string) (tab, error) {
	if id == "" {
		return tabs[0], nil
	}
	for _, candidate := range tabs {
		if candidate.ID == id {
			return candidate, nil
		}
	}
	return tab{}, fmt.Errorf("unknown tab %q", id)
}
