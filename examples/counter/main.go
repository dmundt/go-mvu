package main

import (
	"context"
	"crypto/rand"
	"log"
	"log/slog"
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
	err = server.Run(ctx, server.Config{
		Server:          server.New("localhost:8080", handler),
		ShutdownTimeout: 15 * time.Second,
	})
	log.Println("counter stopped")
	return err
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
	runtime, err := web.NewRuntime(web.RuntimeConfig[string, CounterModel, CounterMessage]{
		App:   CounterApp{},
		Store: store.NewMemory[string, CounterModel](),
		Keys:  web.KeyResolverFunc[string](sessionKey),
		View: CounterView{Document: ui.Document{
			HTMX: true, CSRF: true, SwapValidationErrors: true,
		}},
		Theme:  theme.Default,
		Themes: preference,
	})
	if err != nil {
		return nil, err
	}

	// Development diagnostics: Server-Timing + GET /debug/diagnostics.
	// Keep the endpoint internal or authenticated in production.
	diagnostics := middleware.NewDiagnostics(middleware.DiagnosticsOptions{
		SlowThreshold: 50 * time.Millisecond,
		ServerTiming:  true,
	})

	mux := http.NewServeMux()
	mux.Handle("GET /debug/diagnostics", diagnostics.Handler())
	mux.Handle("GET /{$}", runtime.Page(web.Result{Mode: web.RenderPage}))
	mux.Handle("POST /theme", preference.Handler("/"))
	if err := registerCounterActions(mux, runtime); err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	policy, err := middleware.NewRequestPolicy(middleware.RequestPolicyOptions{
		MaxBodyBytes: 1 << 20,
		Timeout:      10 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	root := http.NewServeMux()
	root.Handle("GET "+static.Prefix, static.Handler())
	root.Handle("/", middleware.Chain(mux, middleware.NoStore,
		sessions.Wrap, preference.Wrap, csrf.Wrap))
	return middleware.Chain(root,
		middleware.RequestID,
		middleware.SecurityHeaders,
		middleware.RequestLogger(logger),
		diagnostics.Middleware,
		middleware.Recover,
		policy,
	), nil
}

func sessionKey(r *http.Request) (string, error) {
	id, ok := session.ID(r)
	if !ok {
		return "", http.ErrNoCookie
	}
	return id, nil
}
