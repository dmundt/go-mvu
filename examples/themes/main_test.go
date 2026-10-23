package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	newHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	return response
}

func TestDefaultThemeRendersInlineInEveryMode(t *testing.T) {
	for _, mode := range []string{"system", "light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			response := get("/?mode=" + mode)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", response.Code, response.Body)
			}
			content := response.Body.String()
			for _, expected := range []string{
				`data-theme="` + mode + `"`,
				"<style>",
				`class="button button--primary"`,
				"<table>",
				"<caption>Users</caption>",
			} {
				if !strings.Contains(content, expected) {
					t.Errorf("output missing %q", expected)
				}
			}
			for _, forbidden := range []string{
				`rel="stylesheet"`, `style="`, "<script", "sourceMappingURL=",
			} {
				if strings.Contains(content, forbidden) {
					t.Errorf("output contains %q", forbidden)
				}
			}
		})
	}
}
