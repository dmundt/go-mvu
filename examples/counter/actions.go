package main

import "github.com/dmundt/go-mvu/web"

var (
	counterResult = web.Result{
		Mode: web.RenderAuto, Fragment: string(counterRegion), Location: "/",
	}
	counterIncrement = web.PostMessage[CounterMessage]("/counter/increment", Increment{}, counterResult)
	counterDecrement = web.PostMessage[CounterMessage]("/counter/decrement", Decrement{}, counterResult)
	counterReset     = web.PostMessage[CounterMessage]("/counter/reset", Reset{}, counterResult)
)
