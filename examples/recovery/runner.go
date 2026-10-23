package main

import (
	"context"
	"errors"
	"time"
)

type Consumer func(context.Context, Effect) error

// DeliverPending makes one ordered pass. A consumer must return success only
// after its own durable commit, deduplicating Effect.ID in that transaction.
func (l *Ledger) DeliverPending(ctx context.Context, consume Consumer) error {
	effects, err := l.Pending()
	if err != nil {
		return err
	}
	for _, effect := range effects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := consume(ctx, effect); err != nil {
			return err
		}
		if err := l.complete(effect.ID); err != nil {
			return err
		}
	}
	return nil
}

// Run is a service: it retries consumer failures, including ambiguous successes,
// until context cancellation. WAL failures stop it; reopening is required.
func (l *Ledger) Run(
	ctx context.Context, interval time.Duration, consume Consumer, report func(error),
) error {
	if interval <= 0 || consume == nil || report == nil {
		return errors.New("positive retry interval, consumer and error reporter required")
	}
	for {
		err := l.DeliverPending(ctx, consume)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			report(err)
			l.mu.Lock()
			ledgerErr := l.ready()
			l.mu.Unlock()
			if ledgerErr != nil {
				return ledgerErr
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
