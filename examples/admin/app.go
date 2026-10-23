package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/validation"
)

const (
	pageSize    = 5
	memberCount = 18
)

var errUnknownMember = errors.New("unknown dashboard member")

type Member struct {
	ID     int
	Name   string
	Email  string
	Role   string
	Active bool
}

type Editor struct {
	ID   int
	Name string
	Role string
}

type Model struct {
	Members []Member
	Query   string
	Role    string
	Page    int
	Form    validation.Form[Editor]
	Notice  string
	Changes int
}

func (model Model) Filtered() []Member {
	var members []Member
	query := strings.ToLower(model.Query)
	for _, member := range model.Members {
		if (model.Role == "" || member.Role == model.Role) &&
			strings.Contains(strings.ToLower(member.Name+" "+member.Email), query) {
			members = append(members, member)
		}
	}
	return members
}

func (model Model) Visible() []Member {
	members := model.Filtered()
	start := min(max((model.Page-1)*pageSize, 0), len(members))
	return members[start:min(start+pageSize, len(members))]
}

type Message interface{ mvu.Message }

type Browse struct {
	Query string
	Role  string
	Page  int
}

func (Browse) MessageName() string { return "admin.browse" }

type Edit struct{ ID int }

func (Edit) MessageName() string { return "admin.edit" }

type Save struct{ Form validation.Form[Editor] }

func (Save) MessageName() string { return "admin.save" }

type Refresh struct{}

func (Refresh) MessageName() string { return "admin.refresh" }

type Cancel struct{}

func (Cancel) MessageName() string { return "admin.cancel" }

type App struct{}

func (App) Init(context.Context) (Model, []mvu.Command[Message], error) {
	names := []string{"Ada", "Grace", "Barbara", "Margaret", "Alan", "Donald"}
	members := make([]Member, memberCount)
	for index := range members {
		members[index] = Member{
			ID:     index + 1,
			Name:   fmt.Sprintf("%s Example %02d", names[index%len(names)], index+1),
			Email:  fmt.Sprintf("member%02d@example.test", index+1),
			Role:   []string{"Admin", "Editor", "Viewer"}[index%3],
			Active: index%4 != 0,
		}
	}
	return Model{Members: members, Page: 1}, nil, nil
}

func (App) Update(
	_ context.Context, model Model, message Message,
) (Model, []mvu.Command[Message], error) {
	switch message := message.(type) {
	case Browse:
		model.Query, model.Role = message.Query, message.Role
		pages := max(1, (len(model.Filtered())+pageSize-1)/pageSize)
		model.Page = min(max(message.Page, 1), pages)
		model.Form, model.Notice = validation.Form[Editor]{}, ""
	case Edit:
		index := slices.IndexFunc(model.Members, func(member Member) bool {
			return member.ID == message.ID
		})
		if index < 0 {
			return model, nil, errUnknownMember
		}
		member := model.Members[index]
		model.Form = validation.Form[Editor]{
			Value: Editor{ID: member.ID, Name: member.Name, Role: member.Role},
		}
		model.Notice = ""
	case Save:
		index := slices.IndexFunc(model.Members, func(member Member) bool {
			return member.ID == message.Form.Value.ID
		})
		if index < 0 {
			return model, nil, errUnknownMember
		}
		model.Form = message.Form
		if message.Form.Valid {
			// Store snapshots share slices; never mutate the previous snapshot.
			model.Members = slices.Clone(model.Members)
			model.Members[index].Name = message.Form.Value.Name
			model.Members[index].Role = message.Form.Value.Role
			model.Changes++
			model.Notice = "Member saved. Refresh overview to update its snapshot."
			model.Form = validation.Form[Editor]{}
			pages := max(1, (len(model.Filtered())+pageSize-1)/pageSize)
			model.Page = min(model.Page, pages)
		}
	case Cancel:
		model.Form = validation.Form[Editor]{}
	case Refresh:
	default:
		return model, nil, errors.New("unsupported dashboard message")
	}
	return model, nil, nil
}
