package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/examples/internal/themepicker"
)

func get(target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	newHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	return response
}

func TestAllThemesRenderInlineInBothModes(t *testing.T) {
	for _, name := range themepicker.Names {
		for _, mode := range []string{"light", "dark"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				response := get("/?theme=" + name + "&mode=" + mode)
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
}

func TestUnknownThemeReturnsBadRequest(t *testing.T) {
	if response := get("/?theme=unknown"); response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}
