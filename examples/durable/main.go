package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/dmundt/go-mvu/internal/example/server"
)

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
