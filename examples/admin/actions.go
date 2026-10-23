package main

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/validation"
	"github.com/dmundt/go-mvu/web"
)

const (
	workspaceRegion ui.Region = "admin-workspace"
	overviewRegion  ui.Region = "admin-overview"
)

var (
	browseAction = web.Action[Message]{
		Method: htmx.Get,
		Path:   "/admin",
		Decode: web.DecoderFunc[Message](decodeBrowse),
		Result: web.Result{Mode: web.RenderAuto, Fragment: string(workspaceRegion)},
	}
	editAction = web.Action[Message]{
		Method: htmx.Get,
		Path:   "/admin/edit",
		Decode: web.DecoderFunc[Message](decodeEdit),
		Result: web.Result{Mode: web.RenderAuto, Fragment: string(workspaceRegion)},
	}
	saveAction = web.Action[Message]{
		Method: htmx.Post,
		Path:   "/admin/save",
		Decode: web.DecoderFunc[Message](decodeSave),
		Result: web.Result{
			Mode: web.RenderAuto, Fragment: string(workspaceRegion), Location: "/admin/view",
		},
	}
	overviewAction = web.PostMessage[Message]("/admin/overview", Refresh{},
		web.Result{Mode: web.RenderAuto, Fragment: string(overviewRegion), Location: "/admin/view"})
)

func badInput() error {
	return &httpx.Error{Status: http.StatusBadRequest, Public: "Invalid dashboard request."}
}

func validRole(role string) bool {
	return role == "Admin" || role == "Editor" || role == "Viewer"
}

func decodeBrowse(request *http.Request) (Message, error) {
	query := request.URL.Query()
	page := 1
	if query.Get("page") != "" {
		var err error
		page, err = strconv.Atoi(query.Get("page"))
		if err != nil {
			return nil, badInput()
		}
	}
	role := query.Get("role")
	if role != "" && !validRole(role) {
		return nil, badInput()
	}
	return Browse{Query: strings.TrimSpace(query.Get("q")), Role: role, Page: page}, nil
}

func decodeEdit(request *http.Request) (Message, error) {
	id, err := strconv.Atoi(request.URL.Query().Get("id"))
	if err != nil || id < 1 || id > memberCount {
		return nil, badInput()
	}
	return Edit{ID: id}, nil
}

func decodeSave(request *http.Request) (Message, error) {
	if err := request.ParseForm(); err != nil {
		return nil, err
	}
	id, err := strconv.Atoi(request.PostForm.Get("id"))
	if err != nil || id < 1 || id > memberCount {
		return nil, badInput()
	}
	value := Editor{
		ID:   id,
		Name: strings.TrimSpace(request.PostForm.Get("name")),
		Role: request.PostForm.Get("role"),
	}
	errs := validation.Errors{}
	length := utf8.RuneCountInString(value.Name)
	if length < 2 || length > 80 {
		errs.Add("name", "Use 2 to 80 characters.")
	}
	if !validRole(value.Role) {
		errs.Add("role", "Choose a valid role.")
	}
	return Save{Form: validation.Form[Editor]{
		Value: value, Errors: errs, Dirty: true, Valid: errs.Empty(),
	}}, nil
}
