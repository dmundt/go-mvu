package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCustomThemeEmbedsAdapterAndUsesSemanticComponents(t *testing.T) {
	response := httptest.NewRecorder()
	newHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body)
	}
	body := response.Body.String()
	for _, want := range []string{
		`data-theme-name="orbit"`,
		`<style>`,
		`--orbit-accent: #72e0c0`,
		`html[data-theme-name="orbit"] .button--primary`,
		`class="button button--primary"`,
		`class="card card--primary"`,
		`class="input"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing %q", want)
		}
	}
	for _, unwanted := range []string{`rel="stylesheet"`, `style="`, "<script"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("response includes %q", unwanted)
		}
	}
}
