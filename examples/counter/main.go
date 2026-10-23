package main

import (
	"context"
	"crypto/rand"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/store"
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
	log.Println("counter listening on http://localhost:8080")
	err = serve(ctx, &http.Server{
		Addr:              "localhost:8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	})
	log.Println("counter stopped")
	return err
}

func serve(ctx context.Context, server *http.Server) error {
	served := make(chan error, 1)
	go func() { served <- server.ListenAndServe() }()
	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	err := server.Shutdown(shutdown)
	if err != nil {
		err = errors.Join(err, server.Close())
	}
	serveErr := <-served
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(err, serveErr)
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
	document := components.PagePolicy{HTMX: true, CSRF: true, SwapValidationErrors: true}
	if err := document.Validate(); err != nil {
		return nil, err
	}
	runtime, err := web.NewRuntime(web.RuntimeConfig[string, CounterModel, CounterMessage]{
		App:   CounterApp{},
		Store: store.NewMemory[string, CounterModel](),
		Keys:  sessionKey,
		View:  CounterView{Document: document},
		Theme: theme.Default,
	})
	if err != nil {
		return nil, err
	}

	// Development diagnostics: Server-Timing + GET /debug/diagnostics.
	// Keep the endpoint internal or authenticated in production.
	diagnostics := web.NewDiagnostics(web.DiagnosticsOptions{
		SlowThreshold: 50 * time.Millisecond,
		ServerTiming:  true,
	})

	mux := http.NewServeMux()
	mux.Handle("GET /debug/diagnostics", diagnostics.Handler())
	mux.Handle("GET /{$}", runtime.Page(web.Result{Mode: web.RenderPage}))
	mux.Handle("POST /theme", preference.Handler("/"))
	if err := runtime.RegisterAll(mux, counterIncrement, counterDecrement, counterReset); err != nil {
		return nil, err
	}

	root := http.NewServeMux()
	root.Handle("GET "+components.AssetPrefix, components.AssetHandler())
	private := sessions.Wrap(preference.Wrap(csrf.Wrap(mux)))
	root.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		private.ServeHTTP(w, r)
	}))
	return web.SecurityHeaders(
		diagnostics.Middleware(web.Recover(requestLimits(root, 10*time.Second))),
	), nil
}

func sessionKey(r *http.Request) (string, error) {
	id, ok := web.SessionID(r)
	if !ok {
		return "", http.ErrNoCookie
	}
	return id, nil
}
