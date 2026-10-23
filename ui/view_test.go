package ui

import (
	"context"
	"errors"
	"testing"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

func TestRegionViewSharesRenderingAndRequestState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	view := RegionView[string]{
		Region: "greeting",
		Content: func(got context.Context, model string, request web.RequestContext) (html.Component, error) {
			calls++
			if got != ctx || request.Locale != "de" {
				t.Fatal("request context not forwarded")
			}
			return components.Text(model), nil
		},
	}
	request := web.RequestContext{Locale: "de"}
	page, err := view.Render(ctx, "<name>", request)
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := view.Fragment(ctx, "greeting", "<name>", request)
	if err != nil {
		t.Fatal(err)
	}
	if got := render(t, page); got != render(t, fragment) ||
		got != `<section id="greeting">&lt;name&gt;</section>` || calls != 2 {
		t.Fatalf("render = %s, calls = %d", got, calls)
	}
	if _, err := view.Fragment(ctx, "missing", "", request); !errors.Is(err, web.ErrUnknownFragment) || calls != 2 {
		t.Fatalf("unknown fragment error = %v, calls = %d", err, calls)
	}
}

func TestRegionViewReportsConfigurationAndContentErrors(t *testing.T) {
	failure := errors.New("content failed")
	for _, test := range []struct {
		view RegionView[int]
		want error
	}{
		{RegionView[int]{Region: "bad region"}, ErrInvalidRegion},
		{RegionView[int]{Region: "region"}, ErrMissingContent},
		{RegionView[int]{
			Region: "region",
			Content: func(context.Context, int, web.RequestContext) (html.Component, error) {
				return nil, failure
			},
		}, failure},
	} {
		fragment, err := test.view.Render(context.Background(), 0, web.RequestContext{})
		if fragment != nil || !errors.Is(err, test.want) {
			t.Fatalf("fragment = %v, error = %v, want %v", fragment, err, test.want)
		}
	}
}
