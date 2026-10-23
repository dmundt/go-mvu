package ui

import (
	"context"
	"errors"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

var ErrInvalidRegion = errors.New("ui: region requires a CSS identifier")

// Region is a stable fragment name and root ID. Names start with an ASCII
// letter or underscore, followed by letters, digits, underscores or hyphens.
// Use string(region) as web.Result.Fragment.
type Region string

// Wrap gives body a section root suitable for outerHTML replacement.
func (region Region) Wrap(body html.Component) html.Fragment {
	return regionFragment{region: region, body: body}
}

// Select returns the same rooted fragment used on the page, or an explicit
// unknown-fragment error. It does not render or retain request state.
func (region Region) Select(name string, body html.Component) (html.Fragment, error) {
	if !region.valid() {
		return nil, ErrInvalidRegion
	}
	if name != string(region) {
		return nil, web.ErrUnknownFragment
	}
	return region.Wrap(body), nil
}

func (region Region) valid() bool {
	if region == "" {
		return false
	}
	for index, char := range region {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char == '_' {
			continue
		}
		if index > 0 && (char >= '0' && char <= '9' || char == '-') {
			continue
		}
		return false
	}
	return true
}

type regionFragment struct {
	html.FragmentMarker
	region Region
	body   html.Component
}

func (fragment regionFragment) RenderHTML(ctx context.Context, out *html.Context) error {
	if !fragment.region.valid() {
		return ErrInvalidRegion
	}
	out.Component(ctx, components.Section{
		ID:       string(fragment.region),
		Children: []html.Component{fragment.body},
	})
	return out.Err()
}
