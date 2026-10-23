package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/dmundt/go-mvu/mvu"
)

type CounterModel struct {
	Count       int
	NextEventID uint64
	Outbox      []OutboxEvent
}

type OutboxEvent struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

type CounterMessage interface {
	mvu.Message
	isCounterMessage()
}

type Increment struct{ Key string }

func (Increment) MessageName() string { return "counter.increment" }
func (Increment) isCounterMessage()   {}

type CounterApp struct {
	Wake chan<- string
}

func (CounterApp) Init(context.Context) (CounterModel, []mvu.Command[CounterMessage], error) {
	return CounterModel{}, nil, nil
}

func (app CounterApp) Update(
	_ context.Context,
	model CounterModel,
	message CounterMessage,
) (CounterModel, []mvu.Command[CounterMessage], error) {
	increment, ok := message.(Increment)
	if !ok {
		return model, nil, fmt.Errorf("unsupported message %T", message)
	}
	if increment.Key == "" {
		return model, nil, errors.New("increment requires a resolved session key")
	}
	model.Count++
	model.NextEventID++
	encodedKey := base64.RawURLEncoding.EncodeToString([]byte(increment.Key))
	event := OutboxEvent{
		ID:    fmt.Sprintf("%s:%d", encodedKey, model.NextEventID),
		Kind:  "counter.incremented",
		Count: model.Count,
	}
	model.Outbox = append(model.Outbox, event)
	return model, []mvu.Command[CounterMessage]{
		WakeOutbox{Key: increment.Key, Wake: app.Wake},
	}, nil
}

type WakeOutbox struct {
	Key  string
	Wake chan<- string
}

func (command WakeOutbox) Execute(ctx context.Context, _ mvu.Dispatch[CounterMessage]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if command.Wake == nil {
		return errors.New("outbox wake channel is required")
	}
	select {
	case command.Wake <- command.Key:
	default:
		// The worker also scans durable records periodically; a full wake queue
		// delays delivery but cannot lose the persisted event.
	}
	return nil
}
