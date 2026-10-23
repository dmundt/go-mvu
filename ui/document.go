// Package ui provides optional application-level composition helpers.
// It connects components to the web runtime without widening core dependencies.
package ui

import (
	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/static"
	"github.com/dmundt/go-mvu/web"
)

// Document holds shared page policy. Enable HTMX and CSRF explicitly at app
// initialization; the app must still mount static.Handler and CSRF middleware.
type Document struct {
	Lang                 string
	Head                 html.Component
	HTMX                 bool
	CSRF                 bool
	SwapValidationErrors bool
}

// Page applies shared policy and request-local appearance to a page.
func (document Document) Page(request web.RequestContext, title string, body html.Component) html.Page {
	page := components.Document{
		Lang:                     document.Lang,
		Title:                    title,
		Head:                     document.Head,
		Theme:                    request.Theme,
		Mode:                     request.Mode,
		HTMXSwapValidationErrors: document.HTMX && document.SwapValidationErrors,
		Body:                     body,
	}
	if document.HTMX {
		page.HTMXSource = static.HTMXPath
		page.HTMXIntegrity = static.HTMXIntegrity
	}
	if document.CSRF && document.HTMX {
		page.Body = middleware.CSRFHTMX{Body: body}
	}
	return page
}
