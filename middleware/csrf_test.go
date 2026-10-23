package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/session"
)

func TestCSRFMiddlewareProtectsUnsafeRequestsAndProvidesFormAndHTMXHelpers(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	sessions, err := session.New(secret, session.Options{})
	if err != nil {
		t.Fatal(err)
	}
	protection, err := NewCSRF(secret)
	if err != nil {
		t.Fatal(err)
	}
	handler := sessions.Wrap(protection.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			token, ok := CSRFToken(r.Context())
			if !ok {
				t.Error("CSRF token missing from safe request")
				http.Error(w, "missing token", http.StatusInternalServerError)
				return
			}
			out := html.NewContext(w)
			out.Component(r.Context(), CSRFInput{})
			out.Component(r.Context(), CSRFHTMX{Body: html.ComponentFunc(func(_ context.Context, out *html.Context) error {
				out.Text(token)
				return out.Err()
			})})
			if err := out.Err(); err != nil {
				t.Errorf("render CSRF helpers: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/", nil))
	cookies := get.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("GET cookies = %d, want session cookie", len(cookies))
	}
	if !strings.Contains(get.Body.String(), `name="_csrf"`) ||
		!strings.Contains(get.Body.String(), `hx-headers="`) {
		t.Fatalf("CSRF helpers missing from rendered output: %s", get.Body.String())
	}
	fieldTokenStart := strings.Index(get.Body.String(), `name="_csrf" value="`)
	if fieldTokenStart < 0 {
		t.Fatalf("hidden token field missing: %s", get.Body.String())
	}
	fieldTokenStart += len(`name="_csrf" value="`)
	fieldTokenEnd := strings.IndexByte(get.Body.String()[fieldTokenStart:], '"')
	if fieldTokenEnd < 0 {
		t.Fatalf("hidden token value is not terminated: %s", get.Body.String())
	}
	fieldToken := get.Body.String()[fieldTokenStart : fieldTokenStart+fieldTokenEnd]
	if !strings.Contains(get.Body.String(), `hx-headers="{&#34;X-CSRF-Token&#34;:&#34;`+fieldToken+`&#34;}"`) {
		t.Fatalf("HTMX header does not use form token: %s", get.Body.String())
	}
	token := strings.TrimSpace(get.Body.String())
	start := strings.LastIndex(token, `">`)
	if start < 0 {
		t.Fatalf("unable to locate rendered HTMX token: %s", token)
	}
	token = strings.TrimSuffix(token[start+2:], "</div>")
	if token == "" {
		t.Fatal("rendered CSRF token empty")
	}

	missing := httptest.NewRecorder()
	missingRequest := httptest.NewRequest(http.MethodPost, "/", nil)
	missingRequest.AddCookie(cookies[0])
	handler.ServeHTTP(missing, missingRequest)
	if missing.Code != http.StatusForbidden {
		t.Fatalf("missing-token status = %d, want 403", missing.Code)
	}

	htmxPost := httptest.NewRecorder()
	htmxRequest := httptest.NewRequest(http.MethodPost, "/", nil)
	htmxRequest.AddCookie(cookies[0])
	htmxRequest.Header.Set(CSRFHeaderName, token)
	handler.ServeHTTP(htmxPost, htmxRequest)
	if htmxPost.Code != http.StatusNoContent {
		t.Fatalf("valid HTMX-token status = %d, want 204", htmxPost.Code)
	}

	formPost := httptest.NewRecorder()
	formRequest := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{
		CSRFFieldName: {token},
	}.Encode()))
	formRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formRequest.AddCookie(cookies[0])
	handler.ServeHTTP(formPost, formRequest)
	if formPost.Code != http.StatusNoContent {
		t.Fatalf("valid form-token status = %d, want 204", formPost.Code)
	}
}

func TestCSRFRejectsUnsafeRequestWithoutSession(t *testing.T) {
	protection, err := NewCSRF([]byte(strings.Repeat("s", 32)))
	if err != nil {
		t.Fatal(err)
	}
	handler := protection.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler reached without session-bound token")
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}
