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

var componentMarkers = []string{
	".tabset__tab", ".dialog::backdrop",
	`class="accordion"`, `class="tabset"`, `class="dialog" popover`, `class="dropdown"`,
	`class="tip"`, `class="avatar`, `class="progress-field"`, `class="spinner"`,
	`<time datetime="2025-01-15">`, `data-channel="stable"`,
	`class="skeleton"`, `role="switch"`, `class="radio-group"`, `class="input textarea"`,
	`class="separator separator--labelled"`, `class="kbd"`, `class="empty-state"`,
	`class="stat"`, `class="steps"`,
}

func TestGalleryRendersSharedComponentsInEveryMode(t *testing.T) {
	for _, mode := range []string{"system", "light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			response := get("/?mode=" + mode)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", response.Code, response.Body)
			}
			content := response.Body.String()
			expected := append([]string{
				`data-theme="` + mode + `"`,
				`data-theme-name="default"`,
			}, componentMarkers...)
			for _, marker := range expected {
				if !strings.Contains(content, marker) {
					t.Errorf("output missing %q", marker)
				}
			}
			for _, forbidden := range []string{`rel="stylesheet"`, ` style="`, "<script"} {
				if strings.Contains(content, forbidden) {
					t.Errorf("output contains %q", forbidden)
				}
			}
		})
	}
}

func TestGalleryTabsPreserveModeAndSelectPanel(t *testing.T) {
	content := get("/?mode=dark&tab=activity").Body.String()
	for _, expected := range []string{
		`href="?mode=dark&amp;tab=activity" aria-current="page"`,
		`aria-labelledby="gallery-tabs-tab-activity"`,
		`<input class="input" id="tab" name="tab" type="hidden" value="activity">`,
		"Three deployments finished today.",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("output missing %q", expected)
		}
	}
}

func TestGalleryRejectsUnknownInput(t *testing.T) {
	if response := get("/?tab=unknown"); response.Code != http.StatusBadRequest {
		t.Errorf("unknown tab status = %d, want 400", response.Code)
	}
}
