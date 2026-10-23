package main

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/core"
)

const pageSize = 5

type User struct {
	Name  string
	Email string
}

// syntheticUsers builds a deterministic demo directory of n users.
// No real people or addresses; .test is a reserved TLD.
func syntheticUsers(n int) []User {
	first := []string{"Ada", "Grace", "Edsger", "Barbara", "Donald", "Margaret", "Alan"}
	last := []string{"Lovelace", "Hopper", "Dijkstra", "Liskov", "Knuth", "Hamilton"}
	users := make([]User, n)
	for i := range users {
		f, l := first[i%len(first)], last[(i/len(first))%len(last)]
		users[i] = User{
			Name:  fmt.Sprintf("%s %s", f, l),
			Email: fmt.Sprintf("user%02d@example.test", i+1),
		}
	}
	return users
}

// UsersModel is per-session view state over a shared read-only directory.
type UsersModel struct {
	Users      []User
	Page       int
	PageSize   int
	TotalItems int
}

// Pages returns the page count, at least 1.
func (model UsersModel) Pages() int {
	if model.PageSize <= 0 || model.TotalItems == 0 {
		return 1
	}
	return (model.TotalItems + model.PageSize - 1) / model.PageSize
}

// Visible returns users on the current page.
func (model UsersModel) Visible() []User {
	start := min(max((model.Page-1)*model.PageSize, 0), len(model.Users))
	end := min(start+model.PageSize, len(model.Users))
	return model.Users[start:end]
}

type UsersMessage interface {
	core.Message
	isUsersMessage()
}

// ShowPage selects a 1-based page; Update clamps it into range.
type ShowPage struct{ Page int }

func (ShowPage) MessageName() string { return "users.show-page" }
func (ShowPage) isUsersMessage()     {}

type UsersApp struct {
	Directory []User
}

func (app UsersApp) Init(context.Context) (UsersModel, []core.Command[UsersMessage], error) {
	return UsersModel{
		Users:      app.Directory,
		Page:       1,
		PageSize:   pageSize,
		TotalItems: len(app.Directory),
	}, nil, nil
}

func (UsersApp) Update(
	_ context.Context,
	model UsersModel,
	msg UsersMessage,
) (UsersModel, []core.Command[UsersMessage], error) {
	switch msg := msg.(type) {
	case ShowPage:
		model.Page = min(max(msg.Page, 1), model.Pages())
	}
	return model, nil, nil
}
