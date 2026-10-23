package main

import (
	"context"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/web"
)

const usersRegion ui.Region = "users-results"

type UsersView struct{ Document ui.Document }

func (UsersView) Fragment(
	_ context.Context, name string, model UsersModel, _ web.RequestContext,
) (html.Fragment, error) {
	return usersRegion.Select(name, userResults(model))
}

func (view UsersView) Page(
	_ context.Context,
	model UsersModel,
	request web.RequestContext,
) (html.Page, error) {
	return view.document(request, "Users",
		components.Link{
			Label:   "View reusable user cards",
			URL:     "/users/cards",
			Variant: components.ButtonSecondary,
		},
		usersRegion.Wrap(userResults(model)),
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
		ui.ThemeSwitcher{
			Mode:   request.Mode,
			Return: request.CurrentURL,
		},
	)
	shell := components.AppShell{
		Header: header,
		Main:   components.Column(body...),
	}
	return view.Document.Page(request, title, shell)
}

func userResults(model UsersModel) html.Component {
	visible := model.Visible()
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
		Page:       model.Page,
		PageSize:   model.PageSize,
		TotalItems: model.TotalItems,
		Window:     1,
		BaseURL:    usersPageAction.Path,
		Target:     usersPageAction.HTMX().Target,
		Swap:       usersPageAction.HTMX().Swap,
		PushURL:    true,
		Attrs:      html.Attrs{html.A("hx-sync", "closest main:replace")},
	}
	return components.Column(content, pagination)
}

func userTable(rows []User) html.Component {
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
	}
}

func userCards(cards []User) html.Component {
	children := components.Map(cards, func(user User) html.Component {
		return UserCard{Name: user.Name, Email: user.Email}
	})
	return components.Section{
		ID:       "users-cards",
		Children: []html.Component{components.Column(children...)},
	}
}
