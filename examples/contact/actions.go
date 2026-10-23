package main

import (
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/web"
)

var submitContact = web.Action[ContactMessage]{
	Method: htmx.Post,
	Path:   "/contact",
	Decode: web.DecoderFunc[ContactMessage](decodeContact),
	Result: web.Result{Mode: web.RenderAuto, Fragment: string(contactRegion)},
}
