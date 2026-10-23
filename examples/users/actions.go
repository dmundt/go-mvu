package main

import (
	"net/http"
	"strconv"

	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/web"
)

var usersPageAction = web.Action[UsersMessage]{
	Method: htmx.Get,
	Path:   "/users",
	Decode: web.DecoderFunc[UsersMessage](decodeShowPage),
	Result: web.Result{Mode: web.RenderAuto, Fragment: string(usersRegion)},
}

func decodeShowPage(r *http.Request) (UsersMessage, error) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}
	return ShowPage{Page: page}, nil
}
