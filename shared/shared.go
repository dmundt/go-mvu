// Package shared provides optional, React-style UI components rendered as
// server-side HTML. Interactive behavior relies on native HTML (details,
// popover, form controls) instead of browser JavaScript. Themes style the
// components through framework-neutral classes; see theme/default/shared.css.
package shared

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/dmundt/go-mvu/html"
)

var errMissingID = errors.New("shared: component requires ID")

// validID restricts IDs to values that are safe in HTML id attributes and
// CSS selectors used by HTMX targets.
func validID(id string) error {
	if id == "" {
		return errMissingID
	}
	for index, r := range id {
		letter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if letter || index > 0 && (r >= '0' && r <= '9' || r == '-' || r == '_') {
			continue
		}
		return fmt.Errorf("shared: invalid ID %q", id)
	}
	return nil
}

func validURL(value string, schemes ...string) error {
	if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) ||
		strings.ContainsAny(value, "\x00\r\n\t\f\v\\") {
		return fmt.Errorf("shared: invalid URL %q", value)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("shared: invalid URL %q: %w", value, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "" || scheme == "http" || scheme == "https" {
		return nil
	}
	for _, allowed := range schemes {
		if scheme == allowed {
			return nil
		}
	}
	return fmt.Errorf("shared: unsupported URL scheme %q", parsed.Scheme)
}

func attr(out *html.Context, name, value string) {
	out.Static(" ")
	out.Static(name)
	out.Static(`="`)
	out.Attribute(value)
	out.Static(`"`)
}

func optionalAttr(out *html.Context, name, value string) {
	if value != "" {
		attr(out, name, value)
	}
}

func flag(out *html.Context, name string, enabled bool) {
	if enabled {
		out.Static(" ")
		out.Static(name)
	}
}

func joinIDs(ids ...string) string {
	present := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			present = append(present, id)
		}
	}
	return strings.Join(present, " ")
}

func fieldError(out *html.Context, id, message string) {
	if message == "" {
		return
	}
	out.Static(`<span class="field-error" role="alert" id="`)
	out.Attribute(id)
	out.Static(`">`)
	out.Text(message)
	out.Static("</span>")
}
