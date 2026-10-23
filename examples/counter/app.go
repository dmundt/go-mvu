package main

import (
	"context"

	"github.com/dmundt/go-mvu/core"
)

type CounterModel struct {
	Count int
}

type CounterMessage interface {
	core.Message
	isCounterMessage()
}

type Increment struct{}
type Decrement struct{}
type Reset struct{}

func (Increment) MessageName() string { return "counter.increment" }
func (Decrement) MessageName() string { return "counter.decrement" }
func (Reset) MessageName() string     { return "counter.reset" }

func (Increment) isCounterMessage() {}
func (Decrement) isCounterMessage() {}
func (Reset) isCounterMessage()     {}

type CounterApp struct{}

func (CounterApp) Init(context.Context) (CounterModel, []core.Command[CounterMessage], error) {
	return CounterModel{}, nil, nil
}

func (CounterApp) Update(
	_ context.Context,
	model CounterModel,
	msg CounterMessage,
) (CounterModel, []core.Command[CounterMessage], error) {
	switch msg.(type) {
	case Increment:
		model.Count++
	case Decrement:
		model.Count--
	case Reset:
		model.Count = 0
	}
	return model, nil, nil
}
