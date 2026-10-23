package main

import (
	"context"
	"errors"
	"math"

	"github.com/dmundt/go-mvu/core"
)

type Model struct {
	Count int64
}

type Add struct {
	Amount int64
}

func (Add) MessageName() string { return "add" }

// Persistence belongs to Ledger.Apply, not this pure application.
var application = core.Functions[Model, Add]{
	UpdateFunc: func(ctx context.Context, model Model, msg Add) (Model, []core.Command[Add], error) {
		if err := ctx.Err(); err != nil {
			return model, nil, err
		}
		if msg.Amount < 1 || msg.Amount > 1_000_000 {
			return model, nil, errors.New("amount must be between 1 and 1000000")
		}
		if model.Count > math.MaxInt64-msg.Amount {
			return model, nil, errors.New("counter overflow")
		}
		return Model{Count: model.Count + msg.Amount}, nil, nil
	},
}
