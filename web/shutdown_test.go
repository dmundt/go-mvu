package web

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/core"
)

// Shutdown contract relied on by graceful servers: a Close timeout bounds only
// the wait. Running and queued accepted jobs keep their contexts and finish.
func TestAsyncRunnerCloseTimeoutKeepsAcceptedJobsRunning(t *testing.T) {
	runner, err := NewAsyncCommandRunner[testMessage](1, 1, time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	queuedRan := make(chan struct{})
	running := core.CommandFunc[testMessage](func(
		ctx context.Context, _ core.Dispatch[testMessage],
	) error {
		started <- ctx
		<-release
		return nil
	})
	queued := core.CommandFunc[testMessage](func(context.Context, core.Dispatch[testMessage]) error {
		close(queuedRan)
		return nil
	})
	if err := runner.Run(context.Background(), shutdownCommands(running), nil); err != nil {
		t.Fatal(err)
	}
	jobCtx := <-started
	if err := runner.Run(context.Background(), shutdownCommands(queued), nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := runner.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close error = %v, want DeadlineExceeded", err)
	}
	if err := jobCtx.Err(); err != nil {
		t.Fatalf("Close timeout cancelled running job: %v", err)
	}
	err = runner.Run(context.Background(), shutdownCommands(queued), nil)
	if !errors.Is(err, ErrCommandClosed) {
		t.Fatalf("Run after Close = %v, want ErrCommandClosed", err)
	}
	select {
	case <-queuedRan:
		t.Fatal("queued job ran while worker busy")
	default:
	}
	close(release)
	if err := runner.Close(context.Background()); err != nil {
		t.Fatalf("drain Close = %v", err)
	}
	select {
	case <-queuedRan:
	default:
		t.Fatal("queued accepted job dropped by timed-out Close")
	}
}

func shutdownCommands(command core.Command[testMessage]) []core.Command[testMessage] {
	return []core.Command[testMessage]{command}
}
