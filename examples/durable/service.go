package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dmundt/go-mvu/internal/example/jsonstore"
	"github.com/dmundt/go-mvu/internal/example/server"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/session"
	"github.com/dmundt/go-mvu/static"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

type service struct {
	handler http.Handler
	store   *jsonstore.Store[CounterModel]
	sink    *FileSink
	worker  *outboxWorker
	wakes   chan string
}

func newService(dataDir string) (*service, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	rootSecret, err := loadSecret(filepath.Join(dataDir, "session.key"))
	if err != nil {
		return nil, err
	}
	sessions, err := session.New(deriveSecret(rootSecret, "session"), session.Options{})
	if err != nil {
		return nil, fmt.Errorf("create sessions: %w", err)
	}
	csrf, err := middleware.NewCSRF(deriveSecret(rootSecret, "csrf"))
	if err != nil {
		return nil, fmt.Errorf("create CSRF protection: %w", err)
	}
	preference, err := theme.NewPreference(
		deriveSecret(rootSecret, "theme"),
		theme.PreferenceOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("create theme preference: %w", err)
	}
	modelStore, err := jsonstore.Open[CounterModel](filepath.Join(dataDir, "models"))
	if err != nil {
		return nil, err
	}
	sink, err := NewFileSink(filepath.Join(dataDir, "delivered"))
	if err != nil {
		return nil, closeStoreAfterError(modelStore, err)
	}
	wakes := make(chan string, 64)
	runtime, err := web.NewRuntime(web.RuntimeConfig[string, CounterModel, CounterMessage]{
		App:   CounterApp{Wake: wakes},
		Store: modelStore,
		Keys: web.KeyResolverFunc[string](func(r *http.Request) (string, error) {
			key, ok := session.ID(r)
			if !ok {
				return "", session.ErrNoSession
			}
			return key, nil
		}),
		View:   CounterView{},
		Theme:  theme.Default,
		Themes: preference,
	})
	if err != nil {
		return nil, closeStoreAfterError(modelStore, fmt.Errorf("create web runtime: %w", err))
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", runtime.Page(web.Result{Mode: web.RenderPage}))
	mux.Handle("POST /increment", runtime.Handle(web.Route[CounterMessage]{
		Method:  http.MethodPost,
		Pattern: "/increment",
		Decode: web.DecoderFunc[CounterMessage](func(r *http.Request) (CounterMessage, error) {
			key, ok := session.ID(r)
			if !ok {
				return nil, session.ErrNoSession
			}
			return Increment{Key: key}, nil
		}),
		Result: web.Result{
			Mode: web.RenderAuto, Fragment: "counter", Location: "/",
		},
	}))
	mux.Handle("POST /theme", preference.Handler("/"))
	policy, err := server.NewRequestPolicy(server.RequestPolicyOptions{
		MaxBodyBytes: 1 << 20,
		Timeout:      10 * time.Second,
	})
	if err != nil {
		return nil, closeStoreAfterError(modelStore, fmt.Errorf("create request policy: %w", err))
	}
	root := http.NewServeMux()
	root.Handle("GET "+static.Prefix, static.Handler())
	root.Handle("/", server.NoStore(
		sessions.Wrap(preference.Wrap(csrf.Wrap(mux))),
	))
	handler := middleware.SecurityHeaders(middleware.Recover(policy(root)))
	return &service{
		handler: handler,
		store:   modelStore,
		sink:    sink,
		worker:  &outboxWorker{store: modelStore, sink: sink},
		wakes:   wakes,
	}, nil
}

func closeStoreAfterError(modelStore *jsonstore.Store[CounterModel], cause error) error {
	if err := modelStore.Close(); err != nil {
		return errors.Join(cause, fmt.Errorf("close durable store: %w", err))
	}
	return cause
}
