package main

import (
	"context"
	"crypto/rand"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dmundt/go-mvu/examples/internal/server"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/session"
	"github.com/dmundt/go-mvu/static"
	"github.com/dmundt/go-mvu/store"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	handler, err := newHandler(secret)
	if err != nil {
		return err
	}
	log.Println("admin demo listening on http://localhost:8086/admin")
	return server.Run(ctx, server.Config{
		Server:          server.New("localhost:8086", handler),
		ShutdownTimeout: 15 * time.Second,
	})
}

func newHandler(secret []byte) (http.Handler, error) {
	sessions, err := session.New(secret, session.Options{Secure: false})
	if err != nil {
		return nil, err
	}
	csrf, err := middleware.NewCSRF(secret)
	if err != nil {
		return nil, err
	}
	preference, err := theme.NewPreference(secret, theme.PreferenceOptions{Secure: false})
	if err != nil {
		return nil, err
	}
	document := ui.Document{HTMX: true, CSRF: true}
	if err := document.Validate(); err != nil {
		return nil, err
	}
	runtime, err := web.NewRuntime(web.RuntimeConfig[string, Model, Message]{
		App:   App{},
		Store: store.NewMemory[string, Model](),
		Keys: web.KeyResolverFunc[string](func(request *http.Request) (string, error) {
			id, ok := session.ID(request)
			if !ok {
				return "", http.ErrNoCookie
			}
			return id, nil
		}),
		View:   View{Document: document},
		Theme:  theme.Default,
		Themes: preference,
	})
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("GET /{$}", http.RedirectHandler("/admin", http.StatusSeeOther))
	mux.Handle("GET /admin/view", runtime.Page(web.Result{Mode: web.RenderPage}))
	if err := runtime.RegisterAll(
		mux, browseAction, editAction, saveAction, overviewAction,
	); err != nil {
		return nil, err
	}
	mux.Handle("POST /theme", preference.Handler("/admin/view"))
	policy, err := middleware.NewRequestPolicy(middleware.RequestPolicyOptions{
		MaxBodyBytes: 1 << 20,
		Timeout:      10 * time.Second,
		ContentTypes: []string{"application/x-www-form-urlencoded"},
	})
	if err != nil {
		return nil, err
	}
	root := http.NewServeMux()
	root.Handle("GET "+static.Prefix, static.Handler())
	root.Handle("/", middleware.Chain(mux, middleware.NoStore,
		sessions.Wrap, preference.Wrap, csrf.Wrap))
	return middleware.Chain(root, middleware.RequestID, middleware.SecurityHeaders,
		middleware.Recover, policy), nil
}
