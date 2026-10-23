package main

import (
	"net/http"

	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/web"
)

var (
	counterIncrement = web.Action[CounterMessage]{
		Method: htmx.Post,
		Path:   "/counter/increment",
		Decode: web.StaticMessage[CounterMessage]{Message: Increment{}},
		Result: web.Result{Mode: web.RenderAuto, Fragment: string(counterRegion), Location: "/"},
	}
	counterDecrement = web.Action[CounterMessage]{
		Method: htmx.Post,
		Path:   "/counter/decrement",
		Decode: web.StaticMessage[CounterMessage]{Message: Decrement{}},
		Result: web.Result{Mode: web.RenderAuto, Fragment: string(counterRegion), Location: "/"},
	}
	counterReset = web.Action[CounterMessage]{
		Method: htmx.Post,
		Path:   "/counter/reset",
		Decode: web.StaticMessage[CounterMessage]{Message: Reset{}},
		Result: web.Result{Mode: web.RenderAuto, Fragment: string(counterRegion), Location: "/"},
	}
)

func registerCounterActions(
	mux *http.ServeMux,
	runtime *web.Runtime[string, CounterModel, CounterMessage],
) error {
	for _, action := range [...]web.Action[CounterMessage]{
		counterIncrement, counterDecrement, counterReset,
	} {
		if err := runtime.Register(mux, action); err != nil {
			return err
		}
	}
	return nil
}
