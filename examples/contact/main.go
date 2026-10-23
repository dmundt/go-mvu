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
	log.Println("contact example listening on http://localhost:8084/contact")
	return server.Run(ctx, server.Config{
		Server:          server.New("localhost:8084", handler),
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
	runtime, err := web.NewRuntime(web.RuntimeConfig[string, ContactModel, ContactMessage]{
		App:   ContactApp{},
		Store: store.NewMemory[string, ContactModel](),
		Keys:  web.KeyResolverFunc[string](sessionKey),
		View: ContactView{Document: ui.Document{
			HTMX: true, CSRF: true,
		}},
		Theme: contactTheme{},
	})
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("GET "+submitContact.Path, runtime.Page(web.Result{Mode: web.RenderPage}))
	if err := runtime.Register(mux, submitContact); err != nil {
		return nil, err
	}

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
	root.Handle("/", middleware.Chain(mux, middleware.NoStore, sessions.Wrap, csrf.Wrap))
	return middleware.Chain(root,
		middleware.RequestID,
		middleware.SecurityHeaders,
		middleware.Recover,
		policy,
	), nil
}

func sessionKey(request *http.Request) (string, error) {
	id, ok := session.ID(request)
	if !ok {
		return "", http.ErrNoCookie
	}
	return id, nil
}
