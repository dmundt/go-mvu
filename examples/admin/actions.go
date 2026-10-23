package main

import (
	"fmt"
	"net/http"
	"slices"
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
	editorRegion    ui.Region = "member-editor-content"
	editorPopover             = "member-editor"
)

var (
	browseAction = web.Action[Message]{
		Method: htmx.Get,
		Path:   "/admin",
		Decode: decodeBrowse,
		Result: web.Result{Mode: web.RenderAuto, Fragment: string(workspaceRegion)},
	}
	editAction = web.Action[Message]{
		Method: htmx.Get,
		Path:   "/admin/edit",
		Decode: decodeEdit,
		Result: web.Result{Mode: web.RenderAuto, Fragment: string(editorRegion)},
		Swap:   htmx.InnerHTML,
	}
	saveAction = web.Action[Message]{
		Method: htmx.Post,
		Path:   "/admin/save",
		Decode: decodeSave,
		Result: web.Result{
			Mode: web.RenderAuto, Fragment: string(workspaceRegion), Location: "/admin/view",
		},
	}
	overviewAction = web.PostMessage[Message]("/admin/overview", Refresh{},
		web.Result{Mode: web.RenderAuto, Fragment: string(overviewRegion), Location: "/admin/view"})
	cancelAction = web.Action[Message]{
		Method: htmx.Get, Path: "/admin/cancel",
		Decode: web.StaticMessage[Message](Cancel{}),
		Result: web.Result{Mode: web.RenderAuto, Fragment: string(editorRegion)},
		Swap:   htmx.InnerHTML,
	}
)

// Response policy is request-local; the runtime still owns Update/save/render.
func saveHandler(runtime *web.Runtime[string, Model, Message]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		message, err := decodeSave(request)
		if err != nil {
			httpx.RespondError(w, request, err)
			return
		}
		result := saveAction.Result
		metadata := htmx.FromRequest(request)
		if metadata.IsHTMX && !metadata.IsBoosted && !metadata.IsHistory {
			if !message.(Save).Form.Valid {
				result.Status = http.StatusUnprocessableEntity
				result.Fragment = string(editorRegion)
				htmx.Retarget(w, "#"+string(editorRegion))
				htmx.Reswap(w, htmx.InnerHTML)
			} else {
				result.Fragment = "member-saved"
			}
		}
		runtime.Handle(web.Route[Message]{
			Method: http.MethodPost, Pattern: saveAction.Path,
			Decode: web.StaticMessage[Message](message), Result: result,
		}).ServeHTTP(w, request)
	})
}

func badInput() error {
	return &httpx.Error{Status: http.StatusBadRequest, Public: "Invalid dashboard request."}
}

func validRole(role string) bool {
	return slices.Contains(roles[:], role)
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
	if length < nameMinLength || length > nameMaxLength {
		errs.Add("name", fmt.Sprintf("Use %d to %d characters.", nameMinLength, nameMaxLength))
	}
	if !validRole(value.Role) {
		errs.Add("role", "Choose a valid role.")
	}
	return Save{Form: validation.Form[Editor]{
		Value: value, Errors: errs, Dirty: true, Valid: errs.Empty(),
	}}, nil
}
