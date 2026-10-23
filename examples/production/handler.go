package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/internal/example/jsonstore"
	"github.com/dmundt/go-mvu/internal/example/server"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/session"
	"github.com/dmundt/go-mvu/static"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
	"golang.org/x/oauth2"
)

type application struct {
	handler http.Handler
	auth    *authentication
	models  *jsonstore.Store[Model]
}

func (app *application) Close() error {
	return errors.Join(app.models.Close(), app.auth.records.Close())
}

func newHandler(ctx context.Context, cfg config) (_ *application, result error) {
	base, err := validateOrigin(cfg.BaseURL, cfg.Development, false)
	if err != nil {
		return nil, fmt.Errorf("GOMVU_BASE_URL: %w", err)
	}
	if _, err := validateOrigin(cfg.Issuer, cfg.Development, true); err != nil {
		return nil, fmt.Errorf("OIDC_ISSUER: %w", err)
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.DataDir == "" {
		return nil, errors.New("OIDC client ID/secret and GOMVU_DATA_DIR are required")
	}
	if string(cfg.SessionKey) == string(cfg.CSRFKey) {
		return nil, errors.New("session and CSRF keys must differ")
	}
	sessions, err := session.New(cfg.SessionKey, session.Options{
		Secure:   base.Scheme == "https",
		Lifetime: 8 * time.Hour,
	})
	if err != nil {
		return nil, err
	}
	csrf, err := middleware.NewCSRF(cfg.CSRFKey)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("OIDC client: too many redirects")
			}
			u := *r.URL
			u.RawQuery = ""
			_, err := validateOrigin(u.String(), cfg.Development, true)
			return err
		},
	}
	discovery, cancel := context.WithTimeout(oidc.ClientContext(ctx, client), 15*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(discovery, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery: %w", err)
	}
	endpoint := provider.Endpoint()
	var metadata struct {
		JWKSURL string `json:"jwks_uri"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return nil, fmt.Errorf("OIDC metadata: %w", err)
	}
	for _, raw := range []string{endpoint.AuthURL, endpoint.TokenURL, metadata.JWKSURL} {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("OIDC endpoint: %w", err)
		}
		// OIDC endpoints may include provider-owned query parameters.
		u.RawQuery = ""
		if _, err := validateOrigin(u.String(), cfg.Development, true); err != nil {
			return nil, fmt.Errorf("OIDC endpoint: %w", err)
		}
	}
	models, err := jsonstore.Open[Model](filepath.Join(cfg.DataDir, "models"))
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, models.Close())
		}
	}()
	records, err := jsonstore.Open[identity](filepath.Join(cfg.DataDir, "auth"))
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, records.Close())
		}
	}()
	auth := &authentication{
		sessions: sessions,
		records:  records,
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  base.Scheme + "://" + base.Host + "/oidc/callback",
			Endpoint:     endpoint,
			Scopes:       []string{oidc.ScopeOpenID},
		},
		verifier: provider.VerifierContext(oidc.ClientContext(context.Background(), client),
			&oidc.Config{ClientID: cfg.ClientID}),
		client:   client,
		issuer:   cfg.Issuer,
		attempts: make(map[string]loginAttempt),
	}
	runtime, err := web.NewRuntime(web.RuntimeConfig[string, Model, Message]{
		App:                 App{},
		Store:               models,
		Keys:                web.KeyResolverFunc[string](accountKey),
		View:                View{},
		Theme:               theme.Default,
		AuthorizeTransition: auth.authorize,
	})
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		if err := web.WritePage(r.Context(), w, 200, loginPage()); err != nil {
			slog.ErrorContext(r.Context(), "render login", "error", err)
		}
	})
	mux.HandleFunc("POST /login", auth.begin)
	mux.HandleFunc("GET /oidc/callback", auth.callback)
	private := http.NewServeMux()
	private.Handle("GET /{$}", runtime.Page(web.Result{Mode: web.RenderPage}))
	private.Handle("POST /increment", runtime.Handle(web.Route[Message]{
		Method:  http.MethodPost,
		Pattern: "/increment",
		Decode:  web.StaticMessage[Message]{Message: Message{}},
		Result:  web.Result{Mode: web.RenderAuto, Fragment: "counter", Location: "/"},
	}))
	private.HandleFunc("POST /logout", auth.logout)
	mux.Handle("/", auth.protect(private))
	authenticated := server.NoStore(sessions.Wrap(csrf.Wrap(mux)))
	root := http.NewServeMux()
	root.Handle("GET "+static.Prefix, static.Handler())
	root.Handle("/", authenticated)
	policy, err := server.NewRequestPolicy(server.RequestPolicyOptions{
		MaxBodyBytes: 64 << 10,
		Timeout:      15 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	referrerPolicy := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Referrer-Policy", "no-referrer")
			next.ServeHTTP(w, r)
		})
	}
	handler := middleware.SecurityHeaders(
		referrerPolicy(middleware.Recover(policy(root))),
	)
	return &application{
		auth:   auth,
		models: models,
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host != base.Host {
				httpx.RespondError(w, r, &httpx.Error{Status: 400})
				return
			}
			handler.ServeHTTP(w, r)
		}),
	}, nil
}
