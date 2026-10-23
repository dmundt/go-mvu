package mvu

import "context"

type Message interface {
	MessageName() string
}

type Dispatch[M Message] func(context.Context, M) error

type Command[M Message] interface {
	Execute(context.Context, Dispatch[M]) error
}

type CommandFunc[M Message] func(context.Context, Dispatch[M]) error

func (fn CommandFunc[M]) Execute(ctx context.Context, dispatch Dispatch[M]) error {
	return fn(ctx, dispatch)
}

type Application[Model any, Msg Message] interface {
	Init(context.Context) (Model, []Command[Msg], error)
	Update(context.Context, Model, Msg) (Model, []Command[Msg], error)
}

type Functions[Model any, Msg Message] struct {
	InitFunc   func(context.Context) (Model, []Command[Msg], error)
	UpdateFunc func(context.Context, Model, Msg) (Model, []Command[Msg], error)
}

func (f Functions[Model, Msg]) Init(ctx context.Context) (Model, []Command[Msg], error) {
	if f.InitFunc == nil {
		var zero Model
		return zero, nil, nil
	}
	return f.InitFunc(ctx)
}

func (f Functions[Model, Msg]) Update(ctx context.Context, model Model, msg Msg) (Model, []Command[Msg], error) {
	if f.UpdateFunc == nil {
		return model, nil, nil
	}
	return f.UpdateFunc(ctx, model, msg)
}
