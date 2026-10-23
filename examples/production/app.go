package main

import (
	"context"

	"github.com/dmundt/go-mvu/core"
)

type Model struct {
	Count int `json:"count"`
}

type Message struct{}

func (Message) MessageName() string { return "counter.increment" }

type App struct{}

func (App) Init(context.Context) (Model, []core.Command[Message], error) {
	return Model{}, nil, nil
}

func (App) Update(
	_ context.Context, model Model, _ Message,
) (Model, []core.Command[Message], error) {
	model.Count++
	return model, nil, nil
}
