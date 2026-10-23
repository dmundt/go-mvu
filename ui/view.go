package ui

import (
	"context"
	"errors"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

var ErrMissingContent = errors.New("ui: region view requires a content function")

// RegionView binds one region to a typed content function. Embed it in a view
// to provide Fragment, and use Render to embed the identical root on a page.
type RegionView[Model any] struct {
	Region  Region
	Content func(context.Context, Model, web.RequestContext) (html.Component, error)
}

func (view RegionView[Model]) Render(
	ctx context.Context, model Model, request web.RequestContext,
) (html.Fragment, error) {
	return view.Fragment(ctx, string(view.Region), model, request)
}

func (view RegionView[Model]) Fragment(
	ctx context.Context, name string, model Model, request web.RequestContext,
) (html.Fragment, error) {
	if !view.Region.valid() {
		return nil, ErrInvalidRegion
	}
	if name != string(view.Region) {
		return nil, web.ErrUnknownFragment
	}
	if view.Content == nil {
		return nil, ErrMissingContent
	}
	body, err := view.Content(ctx, model, request)
	if err != nil {
		return nil, err
	}
	return view.Region.Wrap(body), nil
}
