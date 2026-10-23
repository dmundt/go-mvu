package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dmundt/go-mvu/core"
	"github.com/dmundt/go-mvu/web"
)

const (
	pageSize      = 5
	memberCount   = 18
	nameMinLength = 2
	nameMaxLength = 80
	roleAdmin     = "Admin"
	roleEditor    = "Editor"
	roleViewer    = "Viewer"
)

var errUnknownMember = errors.New("unknown dashboard member")
var roles = [...]string{roleAdmin, roleEditor, roleViewer}

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
	Form    web.Form[Editor]
	Notice  string
	Changes int
}

type directoryPage struct {
	Members []Member
	Total   int
	Page    int
}

func (model Model) directory() directoryPage {
	var members []Member
	query := strings.ToLower(model.Query)
	for _, member := range model.Members {
		if (model.Role == "" || member.Role == model.Role) &&
			strings.Contains(strings.ToLower(member.Name+" "+member.Email), query) {
			members = append(members, member)
		}
	}
	page := min(max(model.Page, 1), max(1, (len(members)+pageSize-1)/pageSize))
	start := min((page-1)*pageSize, len(members))
	return directoryPage{
		Members: members[start:min(start+pageSize, len(members))],
		Total:   len(members),
		Page:    page,
	}
}

func (model Model) memberIndex(id int) (int, error) {
	index := slices.IndexFunc(model.Members, func(member Member) bool {
		return member.ID == id
	})
	if index < 0 {
		return 0, errUnknownMember
	}
	return index, nil
}

func (model Model) saveMember(form web.Form[Editor]) (Model, error) {
	index, err := model.memberIndex(form.Value.ID)
	if err != nil {
		return model, err
	}
	model.Form = form
	if !form.Valid {
		return model, nil
	}
	// Store snapshots share slices; never mutate the previous snapshot.
	model.Members = slices.Clone(model.Members)
	model.Members[index].Name = form.Value.Name
	model.Members[index].Role = form.Value.Role
	model.Changes++
	model.Notice = "Member saved. Refresh overview to update its snapshot."
	model.Form = web.Form[Editor]{}
	model.Page = model.directory().Page
	return model, nil
}

type Message interface{ core.Message }

type Browse struct {
	Query string
	Role  string
	Page  int
}

func (Browse) MessageName() string { return "admin.browse" }

type Edit struct{ ID int }

func (Edit) MessageName() string { return "admin.edit" }

type Save struct{ Form web.Form[Editor] }

func (Save) MessageName() string { return "admin.save" }

type Refresh struct{}

func (Refresh) MessageName() string { return "admin.refresh" }

type Cancel struct{}

func (Cancel) MessageName() string { return "admin.cancel" }

type App struct{}

func (App) Init(context.Context) (Model, []core.Command[Message], error) {
	names := []string{"Ada", "Grace", "Barbara", "Margaret", "Alan", "Donald"}
	members := make([]Member, memberCount)
	for index := range members {
		members[index] = Member{
			ID:     index + 1,
			Name:   fmt.Sprintf("%s Example %02d", names[index%len(names)], index+1),
			Email:  fmt.Sprintf("member%02d@example.test", index+1),
			Role:   roles[index%len(roles)],
			Active: index%4 != 0,
		}
	}
	return Model{Members: members, Page: 1}, nil, nil
}

func (App) Update(
	_ context.Context, model Model, message Message,
) (Model, []core.Command[Message], error) {
	switch message := message.(type) {
	case Browse:
		model.Query, model.Role = message.Query, message.Role
		model.Page = message.Page
		model.Page = model.directory().Page
		model.Form, model.Notice = web.Form[Editor]{}, ""
	case Edit:
		index, err := model.memberIndex(message.ID)
		if err != nil {
			return model, nil, err
		}
		member := model.Members[index]
		model.Form = web.Form[Editor]{
			Value: Editor{ID: member.ID, Name: member.Name, Role: member.Role},
		}
		model.Notice = ""
	case Save:
		next, err := model.saveMember(message.Form)
		return next, nil, err
	case Cancel:
		model.Form = web.Form[Editor]{}
	case Refresh:
	default:
		return model, nil, errors.New("unsupported dashboard message")
	}
	return model, nil, nil
}
