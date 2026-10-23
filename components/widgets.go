// Package components provides optional semantic HTML components and native
// widgets. Components escape dynamic content and use framework-neutral classes;
// themes supply presentation and HTMX optionally enhances native navigation.
package components

import (
	"fmt"
	"net/url"
	"strings"
)

func validURL(value string, schemes ...string) error {
	if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) ||
		strings.ContainsAny(value, "\x00\r\n\t\f\v\\") {
		return fmt.Errorf("components: invalid URL %q", value)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("components: invalid URL %q: %w", value, err)
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
	return fmt.Errorf("components: unsupported URL scheme %q", parsed.Scheme)
}
