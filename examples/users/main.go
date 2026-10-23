// Command users serves a paginated user table and reusable user cards on
// the MVU runtime. GET /users?page=N dispatches ShowPage; HTMX requests get
// the results fragment, plain requests the full page.
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
	"github.com/dmundt/go-mvu/core"
	"github.com/dmundt/go-mvu/internal/example/server"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run serves until SIGINT/SIGTERM, then shuts down gracefully. A second
// signal terminates immediately. Commands are synchronous (no effects), so
// no command runner needs draining.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop)
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	handler, err := newHandler(secret)
	if err != nil {
		return err
	}
	log.Println("users example listening on http://localhost:8081/users?page=1")
	err = server.Run(ctx, server.Config{
		Server:          server.New("localhost:8081", handler),
		ShutdownTimeout: 15 * time.Second,
	})
	log.Println("users example stopped")
	return err
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
	preference, err := theme.NewPreference(secret, theme.PreferenceOptions{Secure: false})
	if err != nil {
		return nil, err
	}
	directory := syntheticUsers(42)
	document := components.PagePolicy{HTMX: true, CSRF: true, SwapValidationErrors: true}
	if err := document.Validate(); err != nil {
		return nil, err
	}
	view := UsersView{Document: document}
	runtime, err := web.NewRuntime(web.RuntimeConfig[string, UsersModel, UsersMessage]{
		App:   UsersApp{Directory: directory},
		Store: core.NewMemory[string, UsersModel](),
		Keys:  sessionKey,
		View:  view,
		Theme: theme.Default,
	})
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	if err := runtime.Register(mux, usersPageAction); err != nil {
		return nil, err
	}
	mux.HandleFunc("GET /users/cards", func(w http.ResponseWriter, r *http.Request) {
		request := web.NewRequestContext(r, theme.Default)
		page := view.cardsPage(request, directory[:6])
		if err := web.WritePage(r.Context(), w, http.StatusOK, page); err != nil {
			log.Printf("render user cards: %v", err)
		}
	})
	mux.Handle("POST /theme", preference.Handler("/users"))

	policy, err := server.NewRequestPolicy(server.RequestPolicyOptions{
		MaxBodyBytes: 1 << 20,
		Timeout:      10 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	root := http.NewServeMux()
	root.Handle("GET "+components.AssetPrefix, components.AssetHandler())
	root.Handle("/", server.NoStore(
		sessions.Wrap(preference.Wrap(csrf.Wrap(mux))),
	))
	return web.SecurityHeaders(web.Recover(policy(root))), nil
}

func sessionKey(r *http.Request) (string, error) {
	id, ok := web.SessionID(r)
	if !ok {
		return "", http.ErrNoCookie
	}
	return id, nil
}
