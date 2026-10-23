package main

import (
	"net/http"
	"testing"
)

func TestHandlerPreservesRoutePolicies(t *testing.T) {
	fixture := newFixture(t, newProvider(t))
	for _, test := range []struct {
		method   string
		path     string
		status   int
		location string
		static   bool
	}{
		{method: http.MethodGet, path: "/login", status: http.StatusOK},
		{method: http.MethodPost, path: "/login", status: http.StatusForbidden},
		{method: http.MethodGet, path: "/oidc/callback", status: http.StatusBadRequest},
		{method: http.MethodGet, path: "/", status: http.StatusSeeOther, location: "/login"},
		{method: http.MethodGet, path: "/missing", status: http.StatusSeeOther, location: "/login"},
		{method: http.MethodGet, path: staticPath, status: http.StatusOK, static: true},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response, _ := fixture.request(t, test.method, test.path, nil, false)
			if response.StatusCode != test.status {
				t.Errorf("status = %d, want %d", response.StatusCode, test.status)
			}
			if got := response.Header.Get("Location"); got != test.location {
				t.Errorf("location = %q, want %q", got, test.location)
			}
			if got := response.Header.Get("Referrer-Policy"); got != "no-referrer" {
				t.Errorf("referrer policy = %q, want no-referrer", got)
			}
			cache := response.Header.Get("Cache-Control")
			if test.static {
				if cache == "" || cache == "no-store" {
					t.Errorf("static asset lost independent cache policy: %q", cache)
				}
				if cookies := response.Cookies(); len(cookies) != 0 {
					t.Errorf("static asset created session cookies: %v", cookies)
				}
			} else if cache != "no-store" {
				t.Errorf("dynamic response cache policy = %q, want no-store", cache)
			}
		})
	}
}
