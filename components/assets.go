package components

import (
	"embed"
	"net/http"
)

// AssetPrefix is the URL path prefix AssetHandler expects to be mounted at.
const AssetPrefix = "/static/"

// HTMXPath is the URL of the embedded HTMX script under AssetPrefix.
const HTMXPath = AssetPrefix + "htmx.min.js"

// HTMXIntegrity is the subresource-integrity hash of htmx.min.js.
const HTMXIntegrity = "sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+"

//go:embed htmx.min.js htmx.LICENSE
var assets embed.FS

// AssetHandler serves embedded HTMX 2.0.4 (0BSD, see htmx.LICENSE) under
// AssetPrefix with immutable caching. Mount at "GET /static/" without an
// additional http.StripPrefix.
func AssetHandler() http.Handler {
	files := http.StripPrefix(AssetPrefix, http.FileServerFS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == AssetPrefix {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		files.ServeHTTP(w, r)
	})
}
