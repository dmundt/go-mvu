package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
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

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dataDir := os.Getenv("GOMVU_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	app, err := newService(dataDir)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		app.worker.Run(workerCtx, app.wakes)
	}()

	httpServer := server.New("localhost:8084", app.handler)
	log.Printf("durable counter listening on http://%s (data: %s)", httpServer.Addr, dataDir)
	return server.Run(ctx, server.Config{
		Server:          httpServer,
		ShutdownTimeout: 5 * time.Second,
		Drain: []func(context.Context) error{func(ctx context.Context) error {
			stopWorker()
			select {
			case <-workerDone:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}},
		Dependencies: []io.Closer{app.store},
	})
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

func loadSecret(path string) ([]byte, error) {
	secret, err := os.ReadFile(path)
	if err == nil {
		if len(secret) != 32 {
			return nil, fmt.Errorf("session key %q must contain exactly 32 bytes", path)
		}
		return secret, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read session key: %w", err)
	}
	secret = make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate session key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadSecret(path)
	}
	if err != nil {
		return nil, fmt.Errorf("create session key: %w", err)
	}
	if _, err := file.Write(secret); err != nil {
		file.Close()
		return nil, fmt.Errorf("write session key: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, fmt.Errorf("sync session key: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close session key: %w", err)
	}
	return secret, nil
}

func deriveSecret(secret []byte, purpose string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("go-mvu durable example: " + purpose))
	return mac.Sum(nil)
}
