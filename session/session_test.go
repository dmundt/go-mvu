package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareSignsAndValidatesSessionCookie(t *testing.T) {
	middleware, err := New([]byte(strings.Repeat("s", 32)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := ID(r)
		if !ok {
			t.Error("session ID missing from request context")
		}
		_, _ = w.Write([]byte(id))
	}))
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))
	cookie := first.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie security options = %+v", cookie)
	}
	second := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(second, request)
	if first.Body.String() != second.Body.String() {
		t.Fatal("valid cookie did not preserve session ID")
	}
}
