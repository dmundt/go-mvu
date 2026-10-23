package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/dmundt/go-mvu/httpx"
)

type Locale string

func LocaleFromContext(ctx context.Context) (Locale, bool) {
	locale := Locale(httpx.Locale(ctx))
	return locale, locale != ""
}

type LocaleSource string

const (
	LocaleCookie         LocaleSource = "cookie"
	LocaleAcceptLanguage LocaleSource = "accept-language"
)

type LocaleOptions struct {
	Supported  []Locale
	Default    Locale
	CookieName string
	// Precedence explicitly orders cookie and Accept-Language sources.
	// The configured default is always the final fallback.
	Precedence []LocaleSource
}

func validLocale(value string) bool {
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	for index, part := range strings.Split(value, "-") {
		if len(part) == 0 || len(part) > 8 {
			return false
		}
		if index == 0 && len(part) < 2 {
			return false
		}
		for _, char := range part {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
				return false
			}
			if index == 0 && char >= '0' && char <= '9' {
				return false
			}
		}
	}
	return true
}

// ResolveLocale resolves only supported tags. Cookie values require exact
// supported tags; Accept-Language supports progressively shorter tag lookup.
func ResolveLocale(options LocaleOptions) (Extension, error) {
	if len(options.Supported) == 0 || len(options.Precedence) == 0 {
		return nil, fmt.Errorf("locale: supported locales and explicit precedence required")
	}
	supported := make(map[string]Locale, len(options.Supported))
	for _, locale := range options.Supported {
		key := strings.ToLower(string(locale))
		if !validLocale(string(locale)) || supported[key] != "" {
			return nil, fmt.Errorf("locale: invalid or duplicate supported tag")
		}
		supported[key] = locale
	}
	fallback, ok := supported[strings.ToLower(string(options.Default))]
	if !ok {
		return nil, fmt.Errorf("locale: default must be supported")
	}
	sources := append([]LocaleSource(nil), options.Precedence...)
	seen := make(map[LocaleSource]bool)
	for _, source := range sources {
		if seen[source] || (source != LocaleCookie && source != LocaleAcceptLanguage) {
			return nil, fmt.Errorf("locale: invalid or duplicate source")
		}
		if source == LocaleCookie && (&http.Cookie{Name: options.CookieName, Value: "x"}).Valid() != nil {
			return nil, fmt.Errorf("locale: valid cookie name required")
		}
		seen[source] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locale := fallback
			for _, source := range sources {
				var candidate Locale
				if source == LocaleCookie {
					httpx.Vary(w.Header(), "Cookie")
					if cookie, err := r.Cookie(options.CookieName); err == nil {
						candidate = supported[strings.ToLower(cookie.Value)]
					}
				} else {
					httpx.Vary(w.Header(), "Accept-Language")
					candidate = matchLanguage(strings.Join(r.Header.Values("Accept-Language"), ","), supported)
				}
				if candidate != "" {
					locale = candidate
					break
				}
			}
			// Vary must include every configured source even when an earlier
			// source wins, so shared caches cannot cross language boundaries.
			if seen[LocaleCookie] {
				httpx.Vary(w.Header(), "Cookie")
			}
			if seen[LocaleAcceptLanguage] {
				httpx.Vary(w.Header(), "Accept-Language")
			}
			next.ServeHTTP(w, r.WithContext(httpx.WithLocale(r.Context(), string(locale))))
		})
	}, nil
}

func matchLanguage(value string, supported map[string]Locale) Locale {
	if len(value) > 4096 {
		return ""
	}
	type preference struct {
		tag string
		q   float64
	}
	var preferences []preference
	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(item, ";")
		tag, q := strings.ToLower(strings.TrimSpace(parts[0])), 1.0
		if !validLocale(tag) {
			continue
		}
		if len(parts) > 2 {
			continue
		}
		if len(parts) == 2 {
			key, val, ok := strings.Cut(strings.TrimSpace(parts[1]), "=")
			if !ok || key != "q" {
				continue
			}
			var err error
			q, err = strconv.ParseFloat(val, 64)
			if err != nil || q <= 0 || q > 1 || q != q {
				continue
			}
		}
		preferences = append(preferences, preference{tag, q})
	}
	sort.SliceStable(preferences, func(i, j int) bool { return preferences[i].q > preferences[j].q })
	for _, preference := range preferences {
		tag := preference.tag
		for {
			if locale := supported[tag]; locale != "" {
				return locale
			}
			index := strings.LastIndexByte(tag, '-')
			if index < 0 {
				break
			}
			tag = tag[:index]
		}
	}
	return ""
}
