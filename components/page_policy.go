package components

import (
	"context"
	"errors"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

var ErrInvalidDocument = errors.New("components: CSRF headers and validation swaps require HTMX")

// PagePolicy holds shared page policy. Enable HTMX and CSRF explicitly at app
// initialization; the app must still mount AssetHandler and web.CSRF middleware.
type PagePolicy struct {
	Lang string
	Head html.Component
	HTMX bool
	// CSRF enables HTMX request headers only, not server-side protection.
	// It requires HTMX and explicitly mounted session and CSRF middleware.
	CSRF                 bool
	SwapValidationErrors bool
}

// Validate rejects settings that would otherwise have no effect. Call during
// startup; Page also rejects invalid configuration before writing any HTML.
func (document PagePolicy) Validate() error {
	if !document.HTMX && (document.CSRF || document.SwapValidationErrors) {
		return ErrInvalidDocument
	}
	return nil
}

// Page applies shared policy and request-local appearance to a page.
func (document PagePolicy) Page(request web.RequestContext, title string, body html.Component) html.Page {
	if err := document.Validate(); err != nil {
		return invalidDocument{err: err}
	}
	page := Document{
		Lang:                     document.Lang,
		Title:                    title,
		Head:                     document.Head,
		Theme:                    request.Theme,
		Mode:                     request.Mode,
		HTMXSwapValidationErrors: document.HTMX && document.SwapValidationErrors,
		Body:                     body,
	}

	if document.HTMX {
		page.HTMXSource = HTMXPath
		page.HTMXIntegrity = HTMXIntegrity
	}
	if document.CSRF && document.HTMX {
		page.Body = CSRFHTMX{Body: body}
	}
	return page
}

type invalidDocument struct {
	html.PageMarker
	err error
}

func (page invalidDocument) RenderHTML(context.Context, *html.Context) error {
	return page.err
}
