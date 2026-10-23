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

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/internal/example/server"
	"github.com/dmundt/go-mvu/store"
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
	sessions, err := web.NewSession(secret, web.SessionOptions{Secure: false})
	if err != nil {
		return nil, err
	}
	csrf, err := web.NewCSRF(secret)
	if err != nil {
		return nil, err
	}
	document := components.PagePolicy{HTMX: true, CSRF: true}
	if err := document.Validate(); err != nil {
		return nil, err
	}
	runtime, err := web.NewRuntime(web.RuntimeConfig[string, ContactModel, ContactMessage]{
		App:   ContactApp{},
		Store: store.NewMemory[string, ContactModel](),
		Keys:  sessionKey,
		View:  contactView(document),
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

	policy, err := server.NewRequestPolicy(server.RequestPolicyOptions{
		MaxBodyBytes: 1 << 20,
		Timeout:      10 * time.Second,
		ContentTypes: []string{"application/x-www-form-urlencoded"},
	})
	if err != nil {
		return nil, err
	}
	root := http.NewServeMux()
	root.Handle("GET "+components.AssetPrefix, components.AssetHandler())
	root.Handle("/", server.NoStore(sessions.Wrap(csrf.Wrap(mux))))
	return web.SecurityHeaders(web.Recover(policy(root))), nil
}

func sessionKey(request *http.Request) (string, error) {
	id, ok := web.SessionID(request)
	if !ok {
		return "", http.ErrNoCookie
	}
	return id, nil
}
