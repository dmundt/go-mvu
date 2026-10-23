package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (err error) {
	path := flag.String("ledger", "recovery.wal", "ledger file (parent must exist)")
	scope := flag.String("scope", "demo", "application model scope")
	id := flag.String("action", "", "stable action ID; omit to only drain pending effects")
	amount := flag.Int64("amount", 1, "counter increment")
	serve := flag.Bool("serve", false, "retry pending effects until interrupted")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ledger, err := Open(*path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, ledger.Close()) }()
	if *id != "" {
		model, duplicate, err := ledger.Apply(ctx, *scope, *id, Add{Amount: *amount})
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout,
			"action result=%d duplicate=%t\n", model.Count, duplicate,
		); err != nil {
			return err
		}
	}
	consume := func(ctx context.Context, effect Effect) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := fmt.Fprintf(os.Stdout,
			"effect id=%s scope=%q total=%d\n", effect.ID, effect.Scope, effect.Total,
		)
		return err
	}
	if !*serve {
		return ledger.DeliverPending(ctx, consume)
	}
	err = ledger.Run(ctx, time.Second, consume, func(err error) {
		fmt.Fprintln(os.Stderr, "effect retry:", err)
	})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
