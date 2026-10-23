package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

var csrfHeader = regexp.MustCompile(`X-CSRF-Token&#34;:&#34;([^&]+)&#34;`)

func TestCounterFlow(t *testing.T) {
	handler, err := newHandler([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	page := fetch(t, client, http.MethodGet, server.URL+"/", "")
	for _, want := range []string{
		`<section id="counter"`, `<output aria-live="polite">0</output>`,
		`hx-post="/counter/increment"`, `hx-post="/counter/decrement"`, `hx-post="/counter/reset"`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("page missing %q", want)
		}
	}
	match := csrfHeader.FindStringSubmatch(page)
	if match == nil {
		t.Fatal("page missing CSRF header")
	}
	token := match[1]

	steps := []struct{ path, count string }{
		{"/counter/increment", "1"},
		{"/counter/increment", "2"},
		{"/counter/decrement", "1"},
		{"/counter/reset", "0"},
	}
	for _, step := range steps {
		fragment := fetch(t, client, http.MethodPost, server.URL+step.path, token)
		if strings.Contains(fragment, "<html") {
			t.Fatalf("%s returned full document", step.path)
		}
		if !strings.Contains(fragment, `<output aria-live="polite">`+step.count+`</output>`) {
			t.Fatalf("%s: want count %s, got %s", step.path, step.count, fragment)
		}
	}
}

func TestCounterRejectsMissingCSRF(t *testing.T) {
	handler, err := newHandler([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/counter/increment", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func fetch(t *testing.T, client *http.Client, method, url, token string) string {
	t.Helper()
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("X-CSRF-Token", token)
		request.Header.Set("HX-Request", "true")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("%s %s: status %d: %s", method, url, response.StatusCode, body)
	}
	return string(body)
}
