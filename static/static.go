// Package static serves framework browser assets from the application origin.
//
// htmx.min.js is HTMX 2.0.4 (0BSD, see htmx.LICENSE), verified against the
// upstream SRI hash HTMXIntegrity. Serving it locally keeps CSP script-src
// 'self' and removes the production CDN dependency.
package static

import (
	"embed"
	"net/http"
)

// Prefix is the URL path prefix Handler expects to be mounted at.
const Prefix = "/static/"

// HTMXPath is the URL of the embedded HTMX script under Prefix.
const HTMXPath = Prefix + "htmx.min.js"

// HTMXIntegrity is the subresource-integrity hash of htmx.min.js.
const HTMXIntegrity = "sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+"

//go:embed htmx.min.js htmx.LICENSE
var assets embed.FS

// Handler serves embedded assets under Prefix with immutable caching.
// Mount it as mux.Handle("GET /static/", static.Handler()).
func Handler() http.Handler {
	files := http.StripPrefix(Prefix, http.FileServerFS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == Prefix {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		files.ServeHTTP(w, r)
	})
}
