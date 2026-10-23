package server_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/internal/example/server"
)

func TestCleanupRemainsOwnedBeyondDeadline(t *testing.T) {
	for _, stage := range []string{"drain", "dependency"} {
		t.Run(stage, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			defer close(release)
			done := make(chan error, 1)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			config := server.Config{
				Server:          server.New("", http.NotFoundHandler()),
				Listener:        listen(t),
				ShutdownTimeout: 10 * time.Millisecond,
			}
			block := func() error {
				close(entered)
				<-release
				return nil
			}
			if stage == "drain" {
				config.Drain = []func(context.Context) error{func(ctx context.Context) error {
					<-ctx.Done()
					return block()
				}}
			} else {
				config.Dependencies = []io.Closer{closerFunc(block)}
			}
			go func() { done <- server.Run(ctx, config) }()
			wait(t, entered, stage)
			select {
			case err := <-done:
				t.Fatalf("cleanup abandoned before release: %v", err)
			case <-time.After(30 * time.Millisecond):
			}
			release <- struct{}{}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("cleanup did not finish after release")
			}
		})
	}
}
