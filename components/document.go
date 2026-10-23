package components

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
)

type Document struct {
	html.PageMarker
	Lang        string
	Title       string
	Description string
	HTMXSource  string
	// HTMXIntegrity is an SRI hash such as components.HTMXIntegrity.
	HTMXIntegrity string
	// HTMXSwapValidationErrors enables swapping 422 responses. Applications
	// must return an appropriate validation fragment for the request target.
	// Other 4xx/5xx responses retain HTMX's no-swap behavior.
	HTMXSwapValidationErrors bool
	Head                     html.Component
	Body                     html.Component
	Theme                    theme.Theme
	Mode                     theme.Mode
}

func (document Document) RenderHTML(ctx context.Context, out *html.Context) error {
	language := document.Lang
	if language == "" {
		language = "en"
	}
	selectedTheme := document.Theme
	if selectedTheme == nil {
		selectedTheme = theme.Default
	}
	out.Static("<!doctype html><html lang=\"")
	out.Attribute(language)
	out.Static("\" data-theme=\"")
	out.Static(document.Mode.String())
	out.Static("\" data-theme-name=\"")
	out.Attribute(selectedTheme.Name())
	if class := theme.DocumentClass(selectedTheme, document.Mode); class != "" {
		out.Static("\" class=\"")
		out.Attribute(class)
	}
	out.Static("\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	out.Static("<title>")
	out.Text(document.Title)
	out.Static("</title>")
	if document.Description != "" {
		out.Static("<meta name=\"description\" content=\"")
		out.Attribute(document.Description)
		out.Static("\">")
	}
	out.Static("<style>")
	out.Static(selectedTheme.Stylesheet())
	out.Static("</style>")
	if document.HTMXSwapValidationErrors {
		out.Static(`<meta name="htmx-config" content="`)
		out.Attribute(`{"responseHandling":[{"code":"204","swap":false},{"code":"422","swap":true},{"code":"[23]..","swap":true},{"code":"[45]..","swap":false,"error":true},{"code":"...","swap":false}]}`)
		out.Static(`">`)
	}
	out.Component(ctx, document.Head)
	if document.HTMXSource != "" {
		if err := validateURL(document.HTMXSource); err != nil {
			return fmt.Errorf("invalid HTMX source URL: %w", err)
		}
		out.Static("<script src=\"")
		out.Attribute(document.HTMXSource)
		out.Static("\"")
		optionalAttribute(out, "integrity", document.HTMXIntegrity)
		out.Static(" defer></script>")
	}
	out.Static("</head><body>")
	out.Component(ctx, document.Body)
	out.Static("</body></html>")
	return out.Err()
}
