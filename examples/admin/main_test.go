package main

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/internal/browsertest"
	"github.com/dmundt/go-mvu/validation"
)

var tokenPattern = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

func TestBrowserServer(t *testing.T) {
	handler, err := newHandler([]byte(strings.Repeat("browser-test", 4)))
	if err != nil {
		t.Fatal(err)
	}
	browsertest.Serve(t, handler)
}

func clientFor(t *testing.T) (*http.Client, string) {
	t.Helper()
	handler, err := newHandler([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}, server.URL
}

func request(
	t *testing.T, client *http.Client, method, target string, values url.Values, enhanced bool,
) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, target, strings.NewReader(values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if enhanced {
		req.Header.Set("HX-Request", "true")
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(body)
}

func TestDashboardWorkflowAndIsolatedRegions(t *testing.T) {
	for _, enhanced := range []bool{false, true} {
		client, base := clientFor(t)
		status, page := request(t, client, http.MethodGet, base+"/admin", nil, false)
		token := tokenPattern.FindStringSubmatch(page)
		if status != http.StatusOK || len(token) != 2 {
			t.Fatalf("initial page status=%d, missing CSRF token", status)
		}
		status, body := request(t, client, http.MethodGet, base+"/admin?q=Ada&role=Admin", nil, enhanced)
		if status != http.StatusOK || !strings.Contains(body, "3 matching members") {
			t.Fatalf("filtered status=%d body=%s", status, body)
		}
		if enhanced && (strings.Contains(body, "<!doctype") ||
			strings.Contains(body, `id="admin-overview"`)) {
			t.Fatal("workspace response included unrelated region or document")
		}
		status, body = request(t, client, http.MethodGet, base+"/admin/edit?id=1", nil, enhanced)
		if status != http.StatusOK || !strings.Contains(body, `value="Ada Example 01"`) {
			t.Fatalf("edit status=%d body=%s", status, body)
		}
		if enhanced && (strings.Contains(body, `id="admin-workspace"`) ||
			strings.Contains(body, `id="member-editor"`) || strings.Contains(body, "<!doctype")) {
			t.Fatal("edit response replaced native shell or unrelated content")
		}
		values := url.Values{
			"_csrf": {token[1]}, "id": {"1"}, "name": {"A"}, "role": {"Admin"},
		}
		status, body = request(t, client, http.MethodPost, base+"/admin/save", values, enhanced)
		wantStatus := http.StatusOK
		if enhanced {
			wantStatus = http.StatusUnprocessableEntity
		}
		if status != wantStatus || !strings.Contains(body, "Use 2 to 80 characters.") ||
			!strings.Contains(body, `value="A"`) {
			t.Fatalf("invalid edit status=%d body=%s", status, body)
		}
		values.Set("name", "Ada <Updated>")
		values.Set("role", "Viewer")
		status, body = request(t, client, http.MethodPost, base+"/admin/save", values, enhanced)
		if status != http.StatusOK || !strings.Contains(body, "Member saved.") {
			t.Fatalf("save status=%d body=%s", status, body)
		}
		if enhanced && (!strings.Contains(body, `hx-swap-oob="innerHTML"`) ||
			!strings.Contains(body, `id="save-status"`) ||
			strings.Contains(body, `value="Ada &lt;Updated&gt;"`)) {
			t.Fatal("save did not replace workspace, close editor, and update status OOB")
		}
		status, body = request(t, client, http.MethodPost, base+"/admin/overview",
			url.Values{"_csrf": {token[1]}}, enhanced)
		if status != http.StatusOK || !strings.Contains(body, "Saved changes") {
			t.Fatalf("overview status=%d body=%s", status, body)
		}
		if enhanced && (!strings.HasPrefix(body, `<section id="admin-overview">`) ||
			strings.Contains(body, `id="admin-workspace"`)) {
			t.Fatal("overview response included unrelated region")
		}
		_, body = request(t, client, http.MethodGet, base+"/admin?q=Updated", nil, enhanced)
		if !strings.Contains(body, "Ada &lt;Updated&gt;") || strings.Contains(body, "Ada <Updated>") {
			t.Fatal("updated name missing or unescaped")
		}
		otherJar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		_, other := request(t, &http.Client{Jar: otherJar}, http.MethodGet,
			base+"/admin?q=Updated", nil, false)
		if !strings.Contains(other, "No matching members") {
			t.Fatal("mutation leaked across visitor sessions")
		}
	}
}

func TestDashboardCancelClearsDraftWithoutSaving(t *testing.T) {
	client, base := clientFor(t)
	request(t, client, http.MethodGet, base+"/admin/edit?id=1", nil, true)
	status, body := request(t, client, http.MethodGet, base+"/admin/cancel", nil, true)
	if status != http.StatusOK || strings.Contains(body, `name="name"`) {
		t.Fatalf("cancel status=%d body=%s", status, body)
	}
	_, body = request(t, client, http.MethodGet, base+"/admin/view", nil, false)
	if strings.Contains(body, `name="name"`) || !strings.Contains(body, "Ada Example 01") {
		t.Fatal("cancel retained draft or changed member")
	}
}

func TestDashboardValidationResponseContract(t *testing.T) {
	client, base := clientFor(t)
	_, page := request(t, client, http.MethodGet, base+"/admin", nil, false)
	token := tokenPattern.FindStringSubmatch(page)
	if len(token) != 2 {
		t.Fatal("missing CSRF token")
	}
	values := url.Values{
		"_csrf": {token[1]}, "id": {"1"}, "name": {"A"}, "role": {"Admin"},
	}
	req, err := http.NewRequest(http.MethodPost, base+"/admin/save",
		strings.NewReader(values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnprocessableEntity ||
		response.Header.Get("HX-Retarget") != "#member-editor-content" ||
		response.Header.Get("HX-Reswap") != "innerHTML" {
		t.Fatalf("validation status=%d headers=%v", response.StatusCode, response.Header)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `aria-describedby="name-error"`) ||
		strings.Contains(string(body), `id="member-editor"`) ||
		strings.Contains(string(body), `hx-swap-oob`) {
		t.Fatal("validation response changed native shell or unrelated regions")
	}
}

func TestDashboardRejectsInvalidRequestsAndCSRF(t *testing.T) {
	client, base := clientFor(t)
	request(t, client, http.MethodGet, base+"/admin", nil, false)
	for _, path := range []string{
		"/admin?page=bad", "/admin?role=Owner", "/admin/edit?id=0", "/admin/edit?id=99",
	} {
		status, _ := request(t, client, http.MethodGet, base+path, nil, false)
		if status != http.StatusBadRequest {
			t.Errorf("%s status=%d", path, status)
		}
	}
	status, _ := request(t, client, http.MethodPost, base+"/admin/save",
		url.Values{"id": {"1"}, "name": {"Changed"}, "role": {"Admin"}}, false)
	if status != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d", status)
	}
}

func TestDashboardUpdatePreservesSnapshotsAndClampsPages(t *testing.T) {
	app := App{}
	model, _, err := app.Init(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	original := model.Members[0]
	next, _, err := app.Update(context.Background(), model, Save{
		Form: validation.Form[Editor]{
			Value: Editor{ID: 1, Name: "Updated", Role: "Viewer"}, Valid: true,
		},
	})
	if err != nil || model.Members[0] != original || next.Members[0].Name != "Updated" {
		t.Fatalf("snapshot mutated or save failed: %v", err)
	}
	filtered, _, err := app.Update(context.Background(), next, Browse{Query: "Updated", Page: 999})
	if err != nil || filtered.Page != 1 || len(filtered.directory().Members) != 1 {
		t.Fatalf("filtered=%+v error=%v", filtered, err)
	}
	invalid, _, err := app.Update(context.Background(), model, Save{
		Form: validation.Form[Editor]{Value: Editor{ID: 1, Name: "A"}, Valid: false},
	})
	if err != nil || invalid.Members[0] != original || invalid.Form.Value.Name != "A" {
		t.Fatalf("invalid form changed member or lost values: %v", err)
	}
}
