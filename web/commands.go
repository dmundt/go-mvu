package web

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/dmundt/go-mvu/mvu"
)

// EffectError marks failures from commands after a model transition committed.
type EffectError struct{ Cause error }

func (e *EffectError) Error() string {
	if e.Cause == nil {
		return "command effect failed after commit"
	}
	return "command effect failed after commit: " + e.Cause.Error()
}

func (e *EffectError) Unwrap() error { return e.Cause }

// CommandRunner executes commands returned by Init and Update.
// dispatch performs a full transition and runs follow-up commands.
type CommandRunner[Msg mvu.Message] interface {
	Run(context.Context, []mvu.Command[Msg], mvu.Dispatch[Msg]) error
}

// SyncCommands runs commands in order before the response renders.
type SyncCommands[Msg mvu.Message] struct{}

func (SyncCommands[Msg]) Run(
	ctx context.Context, commands []mvu.Command[Msg], dispatch mvu.Dispatch[Msg],
) error {
	for _, command := range commands {
		if command == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := command.Execute(ctx, dispatch); err != nil {
			return err
		}
	}
	return nil
}

type asyncTask[Msg mvu.Message] struct {
	command  mvu.Command[Msg]
	dispatch mvu.Dispatch[Msg]
	ctx      context.Context
}

// AsyncCommandRunner bounds concurrent command execution and queued work.
// Jobs retain request context values but not cancellation or deadlines.
// Close stops acceptance and drains queued jobs; a Close context timeout only
// bounds waiting and does not cancel running work. Per-command timeout bounds
// cooperative jobs independently of request and shutdown contexts.
type AsyncCommandRunner[Msg mvu.Message] struct {
	queue   chan asyncTask[Msg]
	timeout time.Duration
	onError func(error)
	mu      sync.Mutex
	closed  bool
	workers sync.WaitGroup
	done    chan struct{}
}

func NewAsyncCommandRunner[Msg mvu.Message](
	concurrency, queueSize int, timeout time.Duration, onError func(error),
) (*AsyncCommandRunner[Msg], error) {
	if concurrency <= 0 || queueSize < 0 || timeout <= 0 {
		return nil, ErrInvalidCommandRunnerConfig
	}
	runner := &AsyncCommandRunner[Msg]{
		queue: make(chan asyncTask[Msg], queueSize), timeout: timeout, onError: onError,
		done: make(chan struct{}),
	}
	runner.workers.Add(concurrency)
	for range concurrency {
		go runner.worker()
	}
	go func() {
		runner.workers.Wait()
		close(runner.done)
	}()
	return runner, nil
}

func (runner *AsyncCommandRunner[Msg]) Run(
	ctx context.Context, commands []mvu.Command[Msg], dispatch mvu.Dispatch[Msg],
) error {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.closed {
		return ErrCommandClosed
	}
	count := 0
	for _, command := range commands {
		if command != nil {
			count++
		}
	}
	if count > cap(runner.queue)-len(runner.queue) {
		if cap(runner.queue) == 0 && count == 1 {
			for _, command := range commands {
				if command == nil {
					continue
				}
				select {
				case runner.queue <- asyncTask[Msg]{
					command: command, dispatch: dispatch, ctx: context.WithoutCancel(ctx),
				}:
					return nil
				default:
					Observe(ctx, CommandOverload)
					return ErrCommandOverload
				}
			}
		}
		Observe(ctx, CommandOverload)
		return ErrCommandOverload
	}
	for _, command := range commands {
		if command == nil {
			continue
		}
		runner.queue <- asyncTask[Msg]{
			command: command, dispatch: dispatch, ctx: context.WithoutCancel(ctx),
		}
	}
	return nil
}

func (runner *AsyncCommandRunner[Msg]) worker() {
	defer runner.workers.Done()
	for task := range runner.queue {
		ctx, cancel := context.WithTimeout(task.ctx, runner.timeout)
		err := task.command.Execute(ctx, task.dispatch)
		cancel()
		if err != nil {
			Observe(ctx, CommittedEffectFailure)
			runner.report(&EffectError{Cause: err})
		}
	}
}

func (runner *AsyncCommandRunner[Msg]) report(err error) {
	if runner.onError != nil {
		runner.onError(err)
		return
	}
	slog.Error("mvu async command failed", "error", err)
}

func (runner *AsyncCommandRunner[Msg]) Close(ctx context.Context) error {
	runner.mu.Lock()
	if !runner.closed {
		runner.closed = true
		close(runner.queue)
	}
	runner.mu.Unlock()
	select {
	case <-runner.done:
		return nil
	default:
	}
	select {
	case <-runner.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
