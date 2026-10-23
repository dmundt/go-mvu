package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func FuzzSessionCookie(f *testing.F) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	sessions, err := NewSession([]byte(strings.Repeat("k", 32)), SessionOptions{Clock: func() time.Time { return now }})
	if err != nil {
		f.Fatal(err)
	}
	valid := sessions.cookie(token{id: strings.Repeat("s", 32), expires: now.Add(DefaultSessionLifetime)}).Value
	f.Add(valid)
	f.Add(valid + "tampered")
	f.Add("")
	f.Add("legacy.signature")
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 8192 {
			t.Skip()
		}
		request := httptest.NewRequest("GET", "/", nil)
		// Set the raw header so malformed cookie bytes exercise parsing rather
		// than net/http's outgoing cookie sanitizer.
		request.Header.Set("Cookie", sessions.options.CookieName+"="+value)
		current, accepted := sessions.readToken(request, now)
		if accepted {
			if current.id != strings.Repeat("s", 32) || !current.expires.Equal(now.Add(DefaultSessionLifetime)) {
				t.Fatalf("invalid cookie accepted: %q", value)
			}
		}
		if _, accepted := sessions.readToken(request, now.Add(DefaultSessionLifetime)); accepted {
			t.Fatal("expired cookie accepted")
		}
	})
}
