package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
)

type authorization struct {
	nonce     string
	challenge string
}

type providerFixture struct {
	server *httptest.Server
	mu     sync.Mutex
	codes  map[string]authorization
	mode   string
}

func newProvider(t *testing.T) *providerFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	p := &providerFixture{codes: make(map[string]authorization)}
	mux := http.NewServeMux()
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	discover := func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"issuer":                                p.server.URL,
			"authorization_endpoint":                p.server.URL + "/authorize",
			"token_endpoint":                        p.server.URL + "/token",
			"jwks_uri":                              p.server.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	}
	mux.HandleFunc("GET /.well-known/openid-configuration", discover)
	mux.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key: &key.PublicKey, KeyID: "fixture", Algorithm: "RS256", Use: "sig",
		}}})
	})
	mux.HandleFunc("GET /authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("client_id") != "fixture-client" || q.Get("code_challenge_method") != "S256" ||
			q.Get("nonce") == "" || q.Get("state") == "" {
			http.Error(w, "invalid authorization request", 400)
			return
		}
		code, err := randomToken()
		if err != nil {
			t.Error(err)
			http.Error(w, "randomness failure", 500)
			return
		}
		p.mu.Lock()
		p.codes[code] = authorization{nonce: q.Get("nonce"), challenge: q.Get("code_challenge")}
		p.mu.Unlock()
		destination, err := url.Parse(q.Get("redirect_uri"))
		if err != nil {
			http.Error(w, "invalid redirect", 400)
			return
		}
		values := destination.Query()
		values.Set("state", q.Get("state"))
		values.Set("code", code)
		destination.RawQuery = values.Encode()
		http.Redirect(w, r, destination.String(), 303)
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", 400)
			return
		}
		clientID, secret, ok := r.BasicAuth()
		if !ok || clientID != "fixture-client" || secret != "fixture-secret" {
			http.Error(w, "bad client", 401)
			return
		}
		p.mu.Lock()
		code, exists := p.codes[r.Form.Get("code")]
		delete(p.codes, r.Form.Get("code"))
		mode := p.mode
		p.mu.Unlock()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !exists || code.challenge != base64.RawURLEncoding.EncodeToString(sum[:]) {
			http.Error(w, "bad PKCE", 400)
			return
		}
		claims := map[string]any{
			"iss":   p.server.URL,
			"sub":   "fixture-user",
			"aud":   "fixture-client",
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"nonce": code.nonce,
		}
		switch mode {
		case "nonce":
			claims["nonce"] = "wrong"
		case "audience":
			claims["aud"] = "another-client"
		case "issuer":
			claims["iss"] = "https://another-issuer.example"
		case "expired":
			claims["exp"] = time.Now().Add(-time.Hour).Unix()
		case "other":
			claims["sub"] = "another-user"
		}
		data, err := json.Marshal(claims)
		if err != nil {
			t.Error(err)
			http.Error(w, "encode", 500)
			return
		}
		signed, err := signer.Sign(data)
		if err != nil {
			t.Error(err)
			http.Error(w, "sign", 500)
			return
		}
		compact, err := signed.CompactSerialize()
		if err != nil {
			t.Error(err)
			http.Error(w, "serialize", 500)
			return
		}
		if mode == "signature" {
			parts := strings.Split(compact, ".")
			parts[2] = base64.RawURLEncoding.EncodeToString(make([]byte, 256))
			compact = strings.Join(parts, ".")
		}
		writeJSON(t, w, map[string]any{
			"access_token": "unused-fixture-token",
			"token_type":   "Bearer",
			"id_token":     compact,
			"expires_in":   3600,
		})
	})
	return p
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Error(err)
	}
}

type fixture struct {
	app     *application
	server  *httptest.Server
	client  *http.Client
	cfg     config
	current atomic.Pointer[application]
}

func newFixture(t *testing.T, provider *providerFixture) *fixture {
	t.Helper()
	server := httptest.NewUnstartedServer(nil)
	cfg := config{
		BaseURL:      "http://" + server.Listener.Addr().String(),
		Issuer:       provider.server.URL,
		ClientID:     "fixture-client",
		ClientSecret: "fixture-secret",
		DataDir:      t.TempDir(),
		SessionKey:   []byte(strings.Repeat("s", 32)),
		CSRFKey:      []byte(strings.Repeat("c", 32)),
		Development:  true,
	}
	app, err := newHandler(context.Background(), cfg)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{app: app, server: server, cfg: cfg, client: &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
	f.current.Store(app)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.current.Load().handler.ServeHTTP(w, r)
	})
	server.Start()
	t.Cleanup(func() {
		server.Close()
		if err := f.app.Close(); err != nil {
			t.Error(err)
		}
	})
	return f
}

func (f *fixture) request(
	t *testing.T, method, path string, form url.Values, htmx bool,
) (*http.Response, string) {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request, err := http.NewRequest(method, f.server.URL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if htmx {
		request.Header.Set("HX-Request", "true")
	}
	response, err := f.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("read: %v close: %v", err, closeErr)
	}
	return response, string(data)
}

var tokenPattern = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

func csrfToken(t *testing.T, body string) string {
	t.Helper()
	match := tokenPattern.FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("missing CSRF input: %s", body)
	}
	return match[1]
}

func (f *fixture) startLogin(t *testing.T) string {
	t.Helper()
	response, body := f.request(t, "GET", "/login", nil, false)
	if response.StatusCode != 200 {
		t.Fatalf("login page: %d %s", response.StatusCode, body)
	}
	response, body = f.request(t, "POST", "/login",
		url.Values{"_csrf": {csrfToken(t, body)}}, false)
	if response.StatusCode != 303 {
		t.Fatalf("login start: %d %s", response.StatusCode, body)
	}
	return response.Header.Get("Location")
}

func (f *fixture) callbackURL(t *testing.T) string {
	t.Helper()
	response, err := f.client.Get(f.startLogin(t))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 303 {
		t.Fatalf("provider authorization: %d", response.StatusCode)
	}
	u, err := url.Parse(response.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return u.RequestURI()
}

func (f *fixture) login(t *testing.T) {
	t.Helper()
	callback := f.callbackURL(t)
	origin, err := url.Parse(f.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	before := f.client.Jar.Cookies(origin)
	response, body := f.request(t, "GET", callback, nil, false)
	if response.StatusCode != 303 || response.Header.Get("Location") != "/" {
		t.Fatalf("callback: %d %s", response.StatusCode, body)
	}
	after := f.client.Jar.Cookies(origin)
	if len(before) == 0 || len(after) == 0 || before[0].Value == after[0].Value {
		t.Fatal("login did not rotate session cookie")
	}
}

func TestLoginPersistenceAndDurableLogout(t *testing.T) {
	p := newProvider(t)
	f := newFixture(t, p)
	response, _ := f.request(t, "GET", "/", nil, false)
	if response.StatusCode != 303 || response.Header.Get("Location") != "/login" {
		t.Fatal("private page not protected")
	}
	f.login(t)
	response, body := f.request(t, "GET", "/", nil, false)
	if response.StatusCode != 200 || !strings.Contains(body, ">0</output>") {
		t.Fatalf("initial counter: %d %s", response.StatusCode, body)
	}
	if response.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("callback referrer policy not preserved")
	}
	response, body = f.request(t, "POST", "/increment",
		url.Values{"_csrf": {csrfToken(t, body)}}, true)
	if response.StatusCode != 200 || strings.Contains(body, "<html") ||
		!strings.Contains(body, ">1</output>") {
		t.Fatalf("HTMX increment: %d %s", response.StatusCode, body)
	}
	response, body = f.request(t, "GET", "/", nil, false)
	response, _ = f.request(t, "POST", "/increment",
		url.Values{"_csrf": {csrfToken(t, body)}}, false)
	if response.StatusCode != 303 || response.Header.Get("Location") != "/" {
		t.Fatal("plain increment must redirect")
	}
	if err := f.app.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := newHandler(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	f.app = reopened
	f.current.Store(reopened)
	_, body = f.request(t, "GET", "/", nil, false)
	if !strings.Contains(body, ">2</output>") {
		t.Fatal("restart lost model or authenticated session")
	}
	origin, _ := url.Parse(f.server.URL)
	stolen := f.client.Jar.Cookies(origin)
	response, _ = f.request(t, "POST", "/logout",
		url.Values{"_csrf": {csrfToken(t, body)}}, false)
	if response.StatusCode != 303 {
		t.Fatalf("logout: %d", response.StatusCode)
	}
	if err := f.app.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err = newHandler(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	f.app = reopened
	f.current.Store(reopened)
	f.client.Jar.SetCookies(origin, stolen)
	response, _ = f.request(t, "GET", "/", nil, false)
	if response.StatusCode != 303 || response.Header.Get("Location") != "/login" {
		t.Fatal("copied cookie resurrected login after restart")
	}
	f.login(t)
	_, body = f.request(t, "GET", "/", nil, false)
	if !strings.Contains(body, ">2</output>") {
		t.Fatal("same OIDC subject did not recover account counter")
	}
}

func TestOIDCTokenRejections(t *testing.T) {
	p := newProvider(t)
	for _, mode := range []string{"nonce", "audience", "issuer", "expired", "signature"} {
		t.Run(mode, func(t *testing.T) {
			p.mu.Lock()
			p.mode = mode
			p.mu.Unlock()
			f := newFixture(t, p)
			response, _ := f.request(t, "GET", f.callbackURL(t), nil, false)
			if response.StatusCode != 401 {
				t.Fatalf("invalid %s: status %d", mode, response.StatusCode)
			}
			response, _ = f.request(t, "GET", "/", nil, false)
			if response.StatusCode != 303 {
				t.Fatal("rejected token authenticated browser")
			}
		})
	}
}

func TestCallbackBindingExpiryAndReplay(t *testing.T) {
	p := newProvider(t)
	f := newFixture(t, p)
	callback := f.callbackURL(t)
	originalJar := f.client.Jar
	otherJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	f.client.Jar = otherJar
	response, _ := f.request(t, "GET", callback, nil, false)
	if response.StatusCode != 400 {
		t.Fatal("callback accepted another session")
	}
	f.client.Jar = originalJar
	response, _ = f.request(t, "GET", callback, nil, false)
	if response.StatusCode != 303 {
		t.Fatal("bound callback failed")
	}
	response, _ = f.request(t, "GET", callback, nil, false)
	if response.StatusCode != 400 {
		t.Fatal("callback replay accepted")
	}
	expired := newFixture(t, p)
	callback = expired.callbackURL(t)
	expired.app.auth.mu.Lock()
	for state, attempt := range expired.app.auth.attempts {
		attempt.Expires = time.Now().Add(-time.Second)
		expired.app.auth.attempts[state] = attempt
	}
	expired.app.auth.mu.Unlock()
	response, _ = expired.request(t, "GET", callback, nil, false)
	if response.StatusCode != 400 {
		t.Fatal("expired callback accepted")
	}
}

func TestCSRFStaticAndRateLimit(t *testing.T) {
	f := newFixture(t, newProvider(t))
	response, _ := f.request(t, "POST", "/login", nil, false)
	if response.StatusCode != 403 {
		t.Fatal("login missing CSRF accepted")
	}

	response, _ = f.request(t, "GET", staticPath, nil, false)
	if response.StatusCode != 200 || len(response.Cookies()) != 0 {
		t.Fatal("static asset created session")
	}
	for range 19 {
		response, _ = f.request(t, "GET", "/login", nil, false)
	}
	response, _ = f.request(t, "GET", "/login", nil, false)
	if response.StatusCode != 429 || response.Header.Get("Retry-After") == "" {
		t.Fatalf("rate-limit: %d", response.StatusCode)
	}
}

func TestPrivateCSRFAndAuthorizationFailure(t *testing.T) {
	f := newFixture(t, newProvider(t))
	f.login(t)
	for _, path := range []string{"/increment", "/logout"} {
		response, _ := f.request(t, "POST", path, nil, false)
		if response.StatusCode != 403 {
			t.Fatalf("%s missing CSRF accepted: %d", path, response.StatusCode)
		}
	}
	anonymous, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	f.client.Jar = anonymous
	response, _ := f.request(t, "GET", "/", nil, true)
	if response.StatusCode != 401 || response.Header.Get("HX-Redirect") != "/login" {
		t.Fatalf("HTMX auth failure: %d %v", response.StatusCode, response.Header)
	}
	if err := f.app.auth.records.Close(); err != nil {
		t.Fatal(err)
	}
	response, _ = f.request(t, "GET", "/", nil, false)
	if response.StatusCode != 500 {
		t.Fatalf("storage failure must not masquerade as logged out: %d", response.StatusCode)
	}
}

func TestAccountLimiterExactThreshold(t *testing.T) {
	f := newFixture(t, newProvider(t))
	f.login(t)
	var response *http.Response
	for range 120 {
		response, _ = f.request(t, "GET", "/", nil, false)
		if response.StatusCode != 200 {
			t.Fatalf("premature account limit: %d", response.StatusCode)
		}
	}
	response, _ = f.request(t, "GET", "/", nil, false)
	if response.StatusCode != 429 || response.Header.Get("Retry-After") == "" {
		t.Fatalf("account limit: %d", response.StatusCode)
	}
}

const staticPath = "/static/htmx.min.js"

func TestAccountIsolationAndGrantExpiry(t *testing.T) {
	p := newProvider(t)
	f := newFixture(t, p)
	f.login(t)
	_, body := f.request(t, "GET", "/", nil, false)
	response, _ := f.request(t, "POST", "/increment",
		url.Values{"_csrf": {csrfToken(t, body)}}, false)
	if response.StatusCode != 303 {
		t.Fatal("increment failed")
	}
	firstJar := f.client.Jar
	secondJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	f.client.Jar = secondJar
	p.mu.Lock()
	p.mode = "other"
	p.mu.Unlock()
	f.login(t)
	_, body = f.request(t, "GET", "/", nil, false)
	if !strings.Contains(body, ">0</output>") {
		t.Fatal("different OIDC subject saw another account's model")
	}
	f.client.Jar = firstJar
	_, body = f.request(t, "GET", "/", nil, false)
	if !strings.Contains(body, ">1</output>") {
		t.Fatal("original account model changed")
	}
	origin, err := url.Parse(f.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range firstJar.Cookies(origin) {
		if cookie.Name != "gomvu_session" {
			continue
		}
		id, err := base64.RawURLEncoding.DecodeString(strings.Split(cookie.Value, ".")[1])
		if err != nil {
			t.Fatal(err)
		}
		current, err := f.app.auth.records.Load(context.Background(), string(id))
		if err != nil {
			t.Fatal(err)
		}
		current.Model.Expires = time.Now().Add(-time.Second)
		if _, err := f.app.auth.records.CompareAndSwap(context.Background(),
			string(id), current.Version, current.Model); err != nil {
			t.Fatal(err)
		}
	}
	response, _ = f.request(t, "GET", "/", nil, false)
	if response.StatusCode != 303 || response.Header.Get("Location") != "/login" {
		t.Fatal("expired durable grant accepted")
	}
}

func TestConfigRejectsUnsafeOrigins(t *testing.T) {
	for _, raw := range []string{
		"http://example.com", "https://user:pass@example.com", "https://example.com/app",
		"https://example.com?query=1", "https://example.com#fragment",
	} {
		if _, err := validateOrigin(raw, false, false); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if _, err := validateOrigin("http://127.0.0.1:8085", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := validateOrigin("http://example.com", true, false); err == nil {
		t.Fatal("development permits remote HTTP")
	}
}

// Opt-in fixture for real-browser verification; never used by the executable.
func TestBrowserFixture(t *testing.T) {
	if os.Getenv("GOMVU_BROWSER_TEST") != "true" {
		t.Skip("set GOMVU_BROWSER_TEST=true for a two-minute browser fixture")
	}
	f := newFixture(t, newProvider(t))
	fmt.Printf("BROWSER_URL=%s\n", f.server.URL)
	time.Sleep(2 * time.Minute)
}
