package main

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/web"
)

const usersRegion ui.Region = "users-results"

type UsersView struct {
	ui.RegionView[UsersModel]
	Document ui.Document
}

func usersContent(
	_ context.Context, model UsersModel, _ web.RequestContext,
) (html.Component, error) {
	return UserResults{Model: model}, nil
}

func (view UsersView) Page(
	ctx context.Context,
	model UsersModel,
	request web.RequestContext,
) (html.Page, error) {
	content, err := view.Render(ctx, model, request)
	if err != nil {
		return nil, err
	}
	return view.document(request, "Users",
		components.Link{
			Label:   "View reusable user cards",
			URL:     "/users/cards",
			Variant: components.ButtonSecondary,
		},
		content,
	), nil
}

// cardsPage is static: no model, no runtime.
func (view UsersView) cardsPage(request web.RequestContext, users []User) html.Page {
	return view.document(request, "User cards",
		components.Breadcrumbs{Items: []components.Breadcrumb{
			{Label: "Users", URL: "/users?page=1"},
			{Label: "Reusable card example"},
		}},
		userCards(users),
	)
}
func (view UsersView) document(
	request web.RequestContext, title string, body ...html.Component,
) html.Page {
	header := components.Row(
		components.Heading{Level: 1, Text: title},
		components.ThemeSwitcher{
			Mode:      request.Mode,
			Return:    request.CurrentURL,
			CSRFField: middleware.CSRFInput{},
		},
	)
	shell := components.AppShell{
		Header: header,
		Main:   components.Column(body...),
	}
	return view.Document.Page(request, title, shell)
}

// UserResults renders the table and pagination inside the shared region.
type UserResults struct {
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
		URL: func(page int) string {
			return fmt.Sprintf("%s?page=%d", usersPageAction.Path, page)
		},
		Target:  usersPageAction.HTMX().Target,
		Swap:    usersPageAction.HTMX().Swap,
		PushURL: true,
		Attrs:   html.Attrs{html.A("hx-sync", "closest main:replace")},
	}
	out.Component(ctx, components.Column(content, pagination))
	return out.Err()
}

// userTable is semantic markup; the theme styles it without classes.
type userTable []User

func (rows userTable) RenderHTML(ctx context.Context, out *html.Context) error {
	body := components.Map(rows, func(user User) html.Component {
		return components.TableRow{Cells: []html.Component{
			components.RowHeader(user.Name),
			components.TextCell(user.Email),
		}}
	})
	return components.Table{
		Caption: "Users",
		Header:  components.HeaderRow{"Name", "Email"},
		Body:    body,
	}.RenderHTML(ctx, out)
}

type userCards []User

func (cards userCards) RenderHTML(ctx context.Context, out *html.Context) error {
	children := components.Map(cards, func(user User) html.Component {
		return UserCard{Name: user.Name, Email: user.Email}
	})
	out.Component(ctx, components.Section{
		ID:       "users-cards",
		Children: []html.Component{components.Column(children...)},
	})
	return out.Err()
}
