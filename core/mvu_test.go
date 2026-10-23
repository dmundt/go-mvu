package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dmundt/go-mvu/core"
)

type increment struct{}

func (increment) MessageName() string { return "counter.increment" }

func TestFunctionsDefaults(t *testing.T) {
	app := core.Functions[int, increment]{}
	model, commands, err := app.Init(context.Background())
	if model != 0 || commands != nil || err != nil {
		t.Fatalf("Init = %d, %v, %v", model, commands, err)
	}
	model, commands, err = app.Update(context.Background(), 7, increment{})
	if model != 7 || commands != nil || err != nil {
		t.Fatalf("Update = %d, %v, %v", model, commands, err)
	}
}

func TestFunctionsDelegate(t *testing.T) {
	var app core.Application[int, increment] = core.Functions[int, increment]{
		InitFunc: func(context.Context) (int, []core.Command[increment], error) {
			return 1, nil, nil
		},
		UpdateFunc: func(_ context.Context, model int, _ increment) (int, []core.Command[increment], error) {
			return model + 1, nil, nil
		},
	}
	model, _, _ := app.Init(context.Background())
	model, _, _ = app.Update(context.Background(), model, increment{})
	if model != 2 {
		t.Fatalf("model = %d, want 2", model)
	}
}

func TestCommandFuncDispatches(t *testing.T) {
	var got []string
	dispatch := core.Dispatch[increment](func(_ context.Context, msg increment) error {
		got = append(got, msg.MessageName())
		return nil
	})
	command := core.CommandFunc[increment](func(ctx context.Context, dispatch core.Dispatch[increment]) error {
		return dispatch(ctx, increment{})
	})
	if err := command.Execute(context.Background(), dispatch); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "counter.increment" {
		t.Fatalf("dispatched %v", got)
	}
}

func TestCommandFuncError(t *testing.T) {
	want := errors.New("boom")
	command := core.CommandFunc[increment](func(context.Context, core.Dispatch[increment]) error {
		return want
	})
	if err := command.Execute(context.Background(), nil); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}
