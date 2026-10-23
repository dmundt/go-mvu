package main

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/static"
	"github.com/dmundt/go-mvu/web"
)

type UsersView struct{}

func (UsersView) Page(
	_ context.Context,
	model UsersModel,
	request web.RequestContext,
) (html.Page, error) {
	return document(request, "Users",
		components.Link{
			Label:   "View reusable user cards",
			URL:     "/users/cards",
			Variant: components.ButtonSecondary,
		},
		UserResults{Model: model},
	), nil
}

func (UsersView) Fragment(
	_ context.Context,
	name string,
	model UsersModel,
	_ web.RequestContext,
) (html.Fragment, error) {
	if name != "results" {
		return nil, web.ErrUnknownFragment
	}
	return UserResults{Model: model}, nil
}

// cardsPage is static: no model, no runtime.
func cardsPage(request web.RequestContext, users []User) components.Document {
	return document(request, "User cards",
		components.Breadcrumbs{Items: []components.Breadcrumb{
			{Label: "Users", URL: "/users?page=1"},
			{Label: "Reusable card example"},
		}},
		userCards(users),
	)
}
func document(
	request web.RequestContext, title string, body ...html.Component,
) components.Document {
	header := components.Inline{Children: []html.Component{
		components.Heading{Level: 1, Text: title},
		components.ThemeSwitcher{
			Mode:      request.Mode,
			Return:    request.CurrentURL,
			CSRFField: middleware.CSRFInput{},
		},
	}}
	shell := components.AppShell{
		Header: header,
		Main:   components.Stack{Children: body},
	}
	return components.Document{
		Title:                    title,
		HTMXSource:               static.HTMXPath,
		HTMXIntegrity:            static.HTMXIntegrity,
		HTMXSwapValidationErrors: true,
		Theme:                    request.Theme,
		Mode:                     request.Mode,
		Body:                     middleware.CSRFHTMX{Body: shell},
	}
}

// UserResults is the HTMX swap target: table plus pagination.
type UserResults struct {
	html.FragmentMarker
	Model UsersModel
}

func (view UserResults) RenderHTML(ctx context.Context, out *html.Context) error {
	visible := view.Model.Visible()
	var content html.Component
	if len(visible) == 0 {
		content = components.EmptyState{
			Title:       "No users",
			Description: "The directory is empty.",
		}
	} else {
		content = userTable(visible)
	}
	pagination := components.Pagination{
		Page:       view.Model.Page,
		PageSize:   view.Model.PageSize,
		TotalItems: view.Model.TotalItems,
		Window:     1,
		URL:        func(page int) string { return fmt.Sprintf("/users?page=%d", page) },
		Target:     "#users-results",
		Swap:       htmx.OuterHTML,
		PushURL:    true,
		Attrs:      html.Attrs{html.A("hx-sync", "closest main:replace")},
	}
	out.Component(ctx, components.Section{
		ID: "users-results",
		Children: []html.Component{components.Stack{
			Children: []html.Component{content, pagination},
		}},
	})
	return out.Err()
}

// userTable is semantic markup; the theme styles it without classes.
type userTable []User

func (rows userTable) RenderHTML(ctx context.Context, out *html.Context) error {
	body := make(components.Sequence, len(rows))
	for i, user := range rows {
		body[i] = components.TableRow{Cells: []html.Component{
			components.TableCell{Header: true, Scope: components.ScopeRow, Body: components.Text(user.Name)},
			components.TableCell{Body: components.Text(user.Email)},
		}}
	}
	return components.Table{
		Caption: "Users",
		Header:  components.HeaderRow{"Name", "Email"},
		Body:    body,
	}.RenderHTML(ctx, out)
}

type userCards []User

func (cards userCards) RenderHTML(ctx context.Context, out *html.Context) error {
	children := make([]html.Component, len(cards))
	for index, user := range cards {
		children[index] = UserCard{Name: user.Name, Email: user.Email}
	}
	out.Component(ctx, components.Section{
		ID:       "users-cards",
		Children: []html.Component{components.Stack{Children: children}},
	})
	return out.Err()
}
