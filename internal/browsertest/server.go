// Package browsertest serves real HTTP handlers for Playwright. It is test
// infrastructure, not an application authoring API.
package browsertest

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func Serve(t *testing.T, handler http.Handler) {
	t.Helper()
	address := os.Getenv("GOMVU_BROWSER_ADDR")
	if address == "" {
		t.Skip("started only by npm run test:browser")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	stopped, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Logf("browser server listening at http://%s", listener.Addr())
	select {
	case err := <-done:
		t.Fatalf("browser server exited unexpectedly: %v", err)
	case <-stopped.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; !errors.Is(err, http.ErrServerClosed) {
			t.Fatal(err)
		}
	}
}
