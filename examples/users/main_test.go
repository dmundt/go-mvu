package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func get(t *testing.T, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	handler, err := newHandler([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, target, nil)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestUsersCardsPageRendersReusableComponent(t *testing.T) {
	response := get(t, "/users/cards", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("response status = %d", response.Code)
	}
	for _, expected := range []string{
		`class="card card--default user-card"`,
		"<h2>Ada Lovelace</h2>",
		"<h2>Grace Lovelace</h2>",
		`data-theme="system"`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("card page missing %q", expected)
		}
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want HTML", got)
	}
}

func TestUsersPageAndFragmentShareTableSemantics(t *testing.T) {
	for _, fragment := range []bool{false, true} {
		var headers map[string]string
		if fragment {
			headers = map[string]string{"HX-Request": "true"}
		}
		response := get(t, "/users?page=1", headers)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", response.Code, response.Body)
		}
		content := response.Body.String()
		for _, expected := range []string{
			`id="users-results"`,
			"<caption>Users</caption>",
			`<th scope="col">Name</th>`,
			`<th scope="row">Ada Lovelace</th>`,
			`<span class="pagination__gap" aria-hidden="true">…</span>`,
		} {
			if !strings.Contains(content, expected) {
				t.Errorf("fragment=%v missing %q", fragment, expected)
			}
		}
		if fragment {
			if strings.Contains(content, "<style>") || strings.Contains(content, "<!doctype") {
				t.Error("fragment repeated document or stylesheet")
			}
		} else {
			for _, expected := range []string{
				"border-bottom: 1px solid var(--border)",
				`src="/static/htmx.min.js"`,
				`name="mode" value="system"`,
			} {
				if !strings.Contains(content, expected) {
					t.Errorf("page missing %q", expected)
				}
			}
		}
	}
}

func TestUsersPageParameterClamps(t *testing.T) {
	for target, want := range map[string]string{
		"/users?page=99":  `aria-current="page">9</span>`,
		"/users?page=-3":  `aria-current="page">1</span>`,
		"/users?page=abc": `aria-current="page">1</span>`,
		"/users?page=5":   `aria-current="page">5</span>`,
	} {
		response := get(t, target, map[string]string{"HX-Request": "true"})
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("%s missing %q", target, want)
		}
	}
}

func TestUpdateClampsAndVisibleSlices(t *testing.T) {
	app := UsersApp{Directory: syntheticUsers(12)}
	model, _, _ := app.Init(context.Background())
	if model.Pages() != 3 {
		t.Fatalf("pages = %d", model.Pages())
	}
	for _, test := range []struct{ page, want, visible int }{
		{-1, 1, 5}, {2, 2, 5}, {3, 3, 2}, {99, 3, 2},
	} {
		next, _, err := app.Update(context.Background(), model, ShowPage{Page: test.page})
		if err != nil {
			t.Fatal(err)
		}
		if next.Page != test.want || len(next.Visible()) != test.visible {
			t.Errorf("page %d → %d (%d visible), want %d (%d)",
				test.page, next.Page, len(next.Visible()), test.want, test.visible)
		}
	}
}

func TestEmptyDirectoryRendersEmptyState(t *testing.T) {
	model, _, _ := UsersApp{}.Init(context.Background())
	var output strings.Builder
	if err := html.Render(context.Background(), &output, results{Model: model}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "No users") || strings.Contains(output.String(), "<table") {
		t.Fatalf("empty render = %s", output.String())
	}
}

func TestSyntheticUsersAreDeterministic(t *testing.T) {
	a, b := syntheticUsers(42), syntheticUsers(42)
	for i := range a {
		if a[i] != b[i] || !strings.HasSuffix(a[i].Email, "@example.test") {
			t.Fatalf("user %d = %+v / %+v", i, a[i], b[i])
		}
	}
}
