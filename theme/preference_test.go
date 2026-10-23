package theme

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newPreference(t *testing.T, options PreferenceOptions) *Preference {
	t.Helper()
	preference, err := NewPreference([]byte(strings.Repeat("t", 32)), options)
	if err != nil {
		t.Fatal(err)
	}
	return preference
}

func modeReader(preference *Preference) http.Handler {
	return preference.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(ModeFromContext(r.Context()).String()))
	}))
}

func postMode(values url.Values, htmx bool) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/theme", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if htmx {
		r.Header.Set("HX-Request", "true")
	}
	return r
}

func TestParseMode(t *testing.T) {
	for value, want := range map[string]Mode{"system": System, "light": Light, "dark": Dark} {
		got, err := ParseMode(value)
		if err != nil || got != want || got.String() != value {
			t.Errorf("ParseMode(%q) = %v, %v", value, got, err)
		}
	}
	for _, invalid := range []string{"", "Dark", "auto", "dark ", "light.x"} {
		if _, err := ParseMode(invalid); err == nil {
			t.Errorf("ParseMode(%q) accepted", invalid)
		}
	}
	if Parse(" DARK ") != Dark || Parse("bogus") != System {
		t.Fatal("lenient Parse mismatch")
	}
}

func TestHandlerPersistsEachModeWithSecureCookie(t *testing.T) {
	preference := newPreference(t, PreferenceOptions{Secure: true})
	handler := preference.Wrap(preference.Handler("/"))
	for _, mode := range Modes {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, postMode(url.Values{ModeField: {mode.String()}, ReturnField: {"/users?page=2"}}, false))
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/users?page=2" {
			t.Fatalf("%s: status %d location %q", mode, w.Code, w.Header().Get("Location"))
		}
		cookie := w.Result().Cookies()[0]
		if cookie.Name != "gomvu_theme" || !cookie.HttpOnly || !cookie.Secure ||
			cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
			t.Fatalf("cookie options = %+v", cookie)
		}
		read := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.AddCookie(cookie)
		modeReader(preference).ServeHTTP(read, request)
		if read.Body.String() != mode.String() {
			t.Fatalf("persisted %q, want %q", read.Body.String(), mode)
		}
	}
}

func TestHandlerHTMXRefreshesShell(t *testing.T) {
	preference := newPreference(t, PreferenceOptions{})
	w := httptest.NewRecorder()
	preference.Handler("/").ServeHTTP(w, postMode(url.Values{ModeField: {"dark"}}, true))
	if w.Code != http.StatusNoContent || w.Header().Get("HX-Refresh") != "true" {
		t.Fatalf("HTMX response = %d, HX-Refresh %q", w.Code, w.Header().Get("HX-Refresh"))
	}
}

func TestHandlerRejectsInvalidModeAndMethod(t *testing.T) {
	preference := newPreference(t, PreferenceOptions{})
	w := httptest.NewRecorder()
	preference.Handler("/").ServeHTTP(w, postMode(url.Values{ModeField: {"neon"}}, false))
	if w.Code != http.StatusBadRequest || len(w.Result().Cookies()) != 0 {
		t.Fatalf("invalid mode: status %d cookies %d", w.Code, len(w.Result().Cookies()))
	}
	w = httptest.NewRecorder()
	preference.Handler("/").ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/theme", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d", w.Code)
	}
}

func TestHandlerBlocksOpenRedirects(t *testing.T) {
	preference := newPreference(t, PreferenceOptions{})
	for _, target := range []string{
		"https://evil.example", "//evil.example", "/\\evil.example", "evil", "/ok\r\nX: y", "javascript:alert(1)",
	} {
		w := httptest.NewRecorder()
		preference.Handler("/home").ServeHTTP(w, postMode(url.Values{ModeField: {"light"}, ReturnField: {target}}, false))
		if got := w.Header().Get("Location"); got != "/home" {
			t.Errorf("return %q redirected to %q", target, got)
		}
	}
	if SafeReturnPath("//x") != "/" || SafeReturnPath("/a?b=c") != "/a?b=c" {
		t.Fatal("SafeReturnPath mismatch")
	}
}

func TestDefaultAndTamperedCookieYieldSystem(t *testing.T) {
	preference := newPreference(t, PreferenceOptions{})
	for _, value := range []string{"", "dark", "dark.invalid", "neon.abc"} {
		w := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		if value != "" {
			request.AddCookie(&http.Cookie{Name: "gomvu_theme", Value: value})
		}
		modeReader(preference).ServeHTTP(w, request)
		if w.Body.String() != "system" {
			t.Errorf("cookie %q mode = %q, want system", value, w.Body.String())
		}
	}
}

func TestToggleHandlerSwitchesLightAndDark(t *testing.T) {
	preference := newPreference(t, PreferenceOptions{})
	toggle := preference.Wrap(preference.ToggleHandler("/\\attacker.example"))
	w := httptest.NewRecorder()
	toggle.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/theme/toggle", nil))
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("toggle = %d %q", w.Code, w.Header().Get("Location"))
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(w.Result().Cookies()[0])
	read := httptest.NewRecorder()
	modeReader(preference).ServeHTTP(read, request)
	if read.Body.String() != "dark" {
		t.Fatalf("toggled from system = %q, want dark", read.Body.String())
	}
}

func TestWeakSecretRejected(t *testing.T) {
	if _, err := NewPreference([]byte("short"), PreferenceOptions{}); err == nil {
		t.Fatal("weak secret accepted")
	}
}
