// Package server runs example HTTP servers with graceful, bounded shutdown.
//
// Shutdown order: stop HTTP admission and drain active requests
// (http.Server.Shutdown), stop and drain command runners (Drain hooks, e.g.
// web.AsyncCommandRunner.Close), then close dependencies in reverse order.
// One ShutdownTimeout budget bounds the whole sequence. Dependencies close
// only when every request and accepted command is known to be finished.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	// ReadHeaderTimeout bounds slow request headers on example servers.
	ReadHeaderTimeout = 5 * time.Second
	// IdleTimeout bounds idle keep-alive connections on example servers.
	IdleTimeout = 60 * time.Second
)

var (
	// ErrInvalidConfig reports a missing Server or non-positive ShutdownTimeout.
	ErrInvalidConfig = errors.New("server: invalid config")
	// ErrDependenciesOpen reports that dependencies were left open because
	// requests or accepted commands may still be running.
	ErrDependenciesOpen = errors.New("server: dependencies left open; work still running")
)

// Config describes one server lifecycle. Run consumes it once: hooks and
// dependencies are invoked at most once per Run call.
type Config struct {
	// Server is served and shut down. Required.
	Server *http.Server
	// Listener is served when set; otherwise Run listens on Server.Addr.
	// Run owns and closes it.
	Listener net.Listener
	// ShutdownTimeout bounds HTTP shutdown, drains, and closers together.
	ShutdownTimeout time.Duration
	// Drain stops admission and waits for accepted background work, in
	// order, after HTTP requests finish. A non-nil error means work may
	// still be running. Typical value: runner.Close.
	Drain []func(context.Context) error
	// Dependencies close in reverse order once everything drained.
	Dependencies []io.Closer
}

// New returns an http.Server with example read-header and idle timeouts.
func New(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: ReadHeaderTimeout,
		IdleTimeout:       IdleTimeout,
	}
}

// Run serves until ctx is cancelled or serving fails, then shuts down.
//
// ctx cancellation (e.g. from signal.NotifyContext) starts shutdown; context
// values carry into the shutdown context, cancellation does not. A serve or
// listen failure also triggers full cleanup and is returned joined with any
// cleanup errors. A clean shutdown returns nil.
//
// When the budget expires, Run force-closes HTTP connections, still calls
// Drain hooks so they stop admitting work, and keeps Dependencies open,
// returning ErrDependenciesOpen joined with the causes. Drain timeouts only
// bound waiting; accepted commands keep running to completion. Run joins its
// serve goroutine before returning and starts no other goroutines.
func Run(ctx context.Context, config Config) error {
	if config.Server == nil || config.ShutdownTimeout <= 0 {
		return ErrInvalidConfig
	}
	listener := config.Listener
	if listener == nil {
		var err error
		listener, err = net.Listen("tcp", config.Server.Addr)
		if err != nil {
			return errors.Join(fmt.Errorf("server: listen: %w", err),
				shutdown(ctx, config, nil))
		}
	}
	served := make(chan error, 1)
	go func() { served <- config.Server.Serve(listener) }()
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-served:
		served = nil
		serveErr = fmt.Errorf("server: serve: %w", serveErr)
	}
	return errors.Join(serveErr, shutdown(ctx, config, served))
}

// shutdown runs the ordered, bounded cleanup sequence. served, when non-nil,
// is the pending Serve result to join after HTTP admission stops.
func shutdown(parent context.Context, config Config, served <-chan error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), config.ShutdownTimeout)
	defer cancel()
	var errs []error
	complete := true
	if err := config.Server.Shutdown(ctx); err != nil {
		complete = false
		errs = append(errs, fmt.Errorf("server: http shutdown: %w", err))
		if err := config.Server.Close(); err != nil {
			errs = append(errs, fmt.Errorf("server: http force close: %w", err))
		}
	}
	if served != nil {
		if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, fmt.Errorf("server: serve: %w", err))
		}
	}
	for _, drain := range config.Drain {
		if drain == nil {
			continue
		}
		if err := drain(ctx); err != nil {
			complete = false
			errs = append(errs, fmt.Errorf("server: drain: %w", err))
		}
	}
	if !complete {
		return errors.Join(append(errs, ErrDependenciesOpen)...)
	}
	for i := len(config.Dependencies) - 1; i >= 0; i-- {
		if dependency := config.Dependencies[i]; dependency != nil {
			if err := dependency.Close(); err != nil {
				errs = append(errs, fmt.Errorf("server: close dependency: %w", err))
			}
		}
	}
	return errors.Join(errs...)
}
