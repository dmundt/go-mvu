package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

func TestDocumentRejectsInactiveConfiguration(t *testing.T) {
	for bits := 0; bits < 8; bits++ {
		document := Document{
			HTMX: bits&1 != 0, CSRF: bits&2 != 0, SwapValidationErrors: bits&4 != 0,
		}
		invalid := !document.HTMX && (document.CSRF || document.SwapValidationErrors)
		if err := document.Validate(); errors.Is(err, ErrInvalidDocument) != invalid {
			t.Fatalf("document %+v: error = %v, invalid = %v", document, err, invalid)
		}
		if !invalid {
			continue
		}
		var output strings.Builder
		page := document.Page(web.RequestContext{}, "Invalid", nil)
		if err := html.Render(context.Background(), &output, page); !errors.Is(err, ErrInvalidDocument) || output.Len() != 0 {
			t.Fatalf("error = %v, output = %q", err, output.String())
		}
	}
}
