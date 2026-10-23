package components

import (
	"context"
	"encoding/json"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

type CSRFInput struct{}

func (CSRFInput) RenderHTML(ctx context.Context, out *html.Context) error {
	token, ok := web.CSRFToken(ctx)
	if !ok {
		return web.ErrCSRFTokenUnset
	}
	out.Static(`<input type="hidden" name="`)
	out.Static(web.CSRFFieldName)
	out.Static(`" value="`)
	out.Attribute(token)
	out.Static(`">`)
	return out.Err()
}

type CSRFHTMX struct {
	Body html.Component
}

func (component CSRFHTMX) RenderHTML(ctx context.Context, out *html.Context) error {
	token, ok := web.CSRFToken(ctx)
	if !ok {
		return web.ErrCSRFTokenUnset
	}
	headers, err := json.Marshal(map[string]string{web.CSRFHeaderName: token})
	if err != nil {
		return err
	}
	out.Static(`<div hx-headers="`)
	out.Attribute(string(headers))
	out.Static(`">`)
	out.Component(ctx, component.Body)
	out.Static(`</div>`)
	return out.Err()
}
