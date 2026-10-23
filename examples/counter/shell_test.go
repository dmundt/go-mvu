package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var csrfField = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

func TestCounterShellStaticThemeAndNoJavaScriptFallback(t *testing.T) {
	handler, err := newHandler([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	response, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	page := string(body)
	csp := response.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("CSP = %q", csp)
	}
	if response.Header.Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID")
	}
	for _, want := range []string{
		`data-theme="system"`, `<header>`, `<aside>`, `<main>`, `<footer>`,
		`src="/static/htmx.min.js" integrity="sha384-`,
		`name="mode" value="dark"`, `action="/counter/increment"`, `name="_csrf"`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("page missing %q", want)
		}
	}
	if strings.Contains(page, "unpkg") {
		t.Fatal("page references CDN")
	}

	asset, err := client.Get(server.URL + "/static/htmx.min.js")
	if err != nil {
		t.Fatal(err)
	}
	asset.Body.Close()
	if asset.StatusCode != http.StatusOK {
		t.Fatalf("static htmx status = %d", asset.StatusCode)
	}

	token := csrfField.FindStringSubmatch(page)
	if token == nil {
		t.Fatal("missing CSRF form field")
	}
	post := func(path string, form url.Values) *http.Response {
		form.Set("_csrf", token[1])
		response, err := client.PostForm(server.URL+path, form)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response
	}
	increment := post("/counter/increment", url.Values{})
	if increment.StatusCode != http.StatusSeeOther || increment.Header.Get("Location") != "/" {
		t.Fatalf("no-JS increment = %d %q", increment.StatusCode, increment.Header.Get("Location"))
	}
	if got := post(
		"/theme", url.Values{"mode": {"dark"}, "return": {"/"}},
	); got.StatusCode != http.StatusSeeOther {
		t.Fatalf("theme post = %d", got.StatusCode)
	}
	page = fetch(t, client, http.MethodGet, server.URL+"/", "")
	if !strings.Contains(page, `data-theme="dark"`) {
		t.Fatal("theme not persisted")
	}
	if !strings.Contains(page, `<output aria-live="polite">1</output>`) {
		t.Fatal("count not persisted")
	}
}
