package web

import (
	"context"
	"errors"

	"github.com/dmundt/go-mvu/core"
)

// dispatcher returns a Dispatch that transitions the model for key and
// hands follow-up commands to the CommandRunner. Commands never run while
// the store holds a lock: CAS completes before Run.
func (runtime *Runtime[K, Model, Msg]) dispatcher(
	key K, expected *core.Version, onCommit func(),
) core.Dispatch[Msg] {
	var followup core.Dispatch[Msg]
	followup = func(ctx context.Context, message Msg) error {
		return runtime.dispatchMessage(ctx, key, message, nil, onCommit, followup)
	}
	return func(ctx context.Context, message Msg) error {
		return runtime.dispatchMessage(ctx, key, message, expected, onCommit, followup)
	}
}

func (runtime *Runtime[K, Model, Msg]) dispatchMessage(
	ctx context.Context, key K, message Msg, expected *core.Version,
	onCommit func(), followup core.Dispatch[Msg],
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	commands, err := runtime.transition(ctx, key, message, expected, onCommit)
	if err != nil {
		return err
	}
	if len(commands) == 0 {
		return nil
	}
	if err := runtime.commands.Run(ctx, commands, followup); err != nil {
		if onCommit != nil {
			return &EffectError{Cause: err}
		}
		return err
	}
	return nil
}

func (runtime *Runtime[K, Model, Msg]) loadOrCreate(
	ctx context.Context, key K,
) (core.Snapshot[Model], []core.Command[Msg], bool, error) {
	for range runtime.maxRetries {
		if err := ctx.Err(); err != nil {
			return core.Snapshot[Model]{}, nil, false, err
		}
		snapshot, err := runtime.store.Load(ctx, key)
		if err == nil {
			return snapshot, nil, false, nil
		}
		if !errors.Is(err, core.ErrNotFound) {
			return core.Snapshot[Model]{}, nil, false, err
		}
		model, commands, err := runtime.app.Init(ctx)
		if err != nil {
			return core.Snapshot[Model]{}, nil, false, err
		}
		if err := ctx.Err(); err != nil {
			return core.Snapshot[Model]{}, nil, false, err
		}
		snapshot, err = runtime.store.Create(ctx, key, model)
		if err == nil {
			return snapshot, commands, true, nil
		}
		if !errors.Is(err, core.ErrAlreadyExists) {
			return core.Snapshot[Model]{}, nil, false, err
		}
	}
	return core.Snapshot[Model]{}, nil, false, core.ErrConflict
}

func (runtime *Runtime[K, Model, Msg]) transition(
	ctx context.Context, key K, message Msg, expected *core.Version, onCommit func(),
) ([]core.Command[Msg], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot, err := runtime.store.Load(ctx, key)
	var initCommands []core.Command[Msg]
	if errors.Is(err, core.ErrNotFound) {
		model, commands, initErr := runtime.app.Init(ctx)
		if initErr != nil {
			return nil, initErr
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if runtime.authorize != nil {
			if authErr := runtime.authorize(ctx, key, model, message); authErr != nil {
				Observe(ctx, AuthorizationRejected)
				return nil, authErr
			}
		}
		snapshot, err = runtime.store.Create(ctx, key, model)
		if errors.Is(err, core.ErrAlreadyExists) {
			snapshot, err = runtime.store.Load(ctx, key)
		} else if err == nil {
			initCommands = commands
			if onCommit != nil {
				onCommit()
			}
		}
	}
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < runtime.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if expected != nil && snapshot.Version != *expected {
			Observe(ctx, StaleAction)
			return nil, staleVersionError()
		}
		if runtime.authorize != nil {
			if err := runtime.authorize(ctx, key, snapshot.Model, message); err != nil {
				Observe(ctx, AuthorizationRejected)
				return nil, err
			}
		}
		model, commands, err := runtime.app.Update(ctx, snapshot.Model, message)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		_, err = runtime.store.CompareAndSwap(ctx, key, snapshot.Version, model)
		if err == nil {
			if onCommit != nil {
				onCommit()
			}
			return append(initCommands, commands...), nil
		}
		if !errors.Is(err, core.ErrConflict) {
			return nil, err
		}
		if expected != nil {
			Observe(ctx, StaleAction)
			return nil, staleVersionError()
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if snapshot, err = runtime.store.Load(ctx, key); err != nil {
			return nil, err
		}
	}
	return nil, core.ErrConflict
}
