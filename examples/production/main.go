// Command production demonstrates durable single-host storage and OIDC login.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("production example stopped", "error", err)
		os.Exit(1)
	}
}

func run() (result error) {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := newHandler(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, app.Close()) }()
	address := os.Getenv("GOMVU_LISTEN")
	if address == "" {
		address = "127.0.0.1:8085"
	}
	server := &http.Server{
		Addr:              address,
		Handler:           app.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	stopped := make(chan error, 1)
	go func() { stopped <- server.ListenAndServe() }()
	slog.Info("production example starting", "address", address, "origin", cfg.BaseURL)
	select {
	case err := <-stopped:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			return errors.Join(err, server.Close())
		}
		if err := <-stopped; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
