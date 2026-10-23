package main

import (
	"crypto/rand"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/session"
	"github.com/dmundt/go-mvu/static"
	"github.com/dmundt/go-mvu/store"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

func main() {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		log.Fatal(err)
	}
	handler, err := newHandler(secret)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("counter listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe("localhost:8080", handler))
}

// counterRoutes maps POST paths to messages.
var counterRoutes = []struct {
	path string
	msg  CounterMessage
}{
	{path: "/counter/increment", msg: Increment{}},
	{path: "/counter/decrement", msg: Decrement{}},
	{path: "/counter/reset", msg: Reset{}},
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
		App:    CounterApp{},
		Store:  store.NewMemory[string, CounterModel](),
		Keys:   web.KeyResolverFunc[string](sessionKey),
		View:   CounterView{},
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
	mux.Handle("GET "+static.Prefix, static.Handler())
	mux.Handle("GET /{$}", runtime.Page(web.Result{Mode: web.RenderPage}))
	mux.Handle("POST /theme", preference.Handler("/"))
	for _, route := range counterRoutes {
		mux.Handle("POST "+route.path, runtime.Handle(web.Route[CounterMessage]{
			Method:  http.MethodPost,
			Pattern: route.path,
			Decode:  web.StaticMessage[CounterMessage]{Message: route.msg},
			// HTMX → fragment; plain form post → redirect home.
			Result: web.Result{Mode: web.RenderAuto, Fragment: "counter", Location: "/"},
		}))
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	var handler http.Handler = mux
	handler = csrf.Wrap(handler)
	handler = preference.Wrap(handler)
	handler = sessions.Wrap(handler)
	handler = diagnostics.Middleware(handler)
	handler = middleware.RequestLogger(logger)(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.SecurityHeaders(handler)
	return middleware.Recover(handler), nil
}

func sessionKey(r *http.Request) (string, error) {
	id, ok := session.ID(r)
	if !ok {
		return "", http.ErrNoCookie
	}
	return id, nil
}
