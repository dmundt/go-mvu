package main

import (
	"log"
	"net/http"
	"time"

	"github.com/dmundt/go-mvu/web"
)

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if err := web.WritePage(r.Context(), w, http.StatusOK, helloPage()); err != nil {
			log.Printf("render hello: %v", err)
		}
	})
	return mux
}

func main() {
	log.Println("hello example listening on http://localhost:8085")
	server := &http.Server{
		Addr: "localhost:8085", Handler: newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
