package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersSetStrictCSP(t *testing.T) {
	w := httptest.NewRecorder()
	SecurityHeaders(http.NotFoundHandler()).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := w.Header().Get("Content-Security-Policy")
	for _, want := range []string{"script-src 'self'", "frame-ancestors 'none'", "object-src 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q: %s", want, csp)
		}
	}
	scriptSrc := csp[strings.Index(csp, "script-src"):]
	scriptSrc = scriptSrc[:strings.Index(scriptSrc, ";")]
	if strings.Contains(scriptSrc, "unsafe") {
		t.Fatalf("script-src allows unsafe sources: %s", scriptSrc)
	}
	for name, want := range map[string]string{
		"Permissions-Policy":     DefaultPermissionsPolicy,
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"X-Frame-Options":        "DENY",
	} {
		if got := w.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestRecoverReturns500(t *testing.T) {
	w := httptest.NewRecorder()
	Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })).
		ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
}
