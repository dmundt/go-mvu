package main

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/shared"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/web"
)

type View struct{ Document ui.Document }

func (view View) Page(
	_ context.Context, model Model, request web.RequestContext,
) (html.Page, error) {
	return view.Document.Page(request, "Operations dashboard", components.AppShell{
		Header: components.Row(
			components.Heading{Level: 1, Text: "Northstar / Operations"},
			components.ThemeSwitcher{
				Mode:      request.Mode,
				Return:    "/admin/view",
				CSRFField: middleware.CSRFInput{},
			},
		),
		Sidebar: components.Navigation{Label: "Administration", Items: []components.NavItem{
			{Label: "Overview", URL: "#admin-overview"},
			{Label: "Members", URL: "#admin-workspace"},
			{Label: "About this demo", URL: "#about-demo"},
		}},
		Main: components.Column(
			overviewRegion.Wrap(overview(model)),
			workspaceRegion.Wrap(workspace(model)),
			components.Section{
				ID: "about-demo",
				Children: []html.Component{components.Card{
					Header: components.Heading{Level: 2, Text: "Built for real workflows"},
					Body: components.Paragraph(
						"Typed Go state, reusable components, validated forms, and independent " +
							"HTML updates. No application JavaScript. " +
							"Demo only: no login; changes are private to your session and lost on restart.",
					),
				}},
			},
		),
		Footer: components.Paragraph("Northstar demo / sample data only / not a production admin system"),
	}), nil
}

func (View) Fragment(
	_ context.Context, name string, model Model, _ web.RequestContext,
) (html.Fragment, error) {
	switch name {
	case string(workspaceRegion):
		return workspaceRegion.Wrap(workspace(model)), nil
	case string(overviewRegion):
		return overviewRegion.Wrap(overview(model)), nil
	default:
		return nil, web.ErrUnknownFragment
	}
}

func overview(model Model) html.Component {
	active, admins := 0, 0
	for _, member := range model.Members {
		if member.Active {
			active++
		}
		if member.Role == roleAdmin {
			admins++
		}
	}
	return components.Column(
		components.Row(
			components.Heading{Level: 2, Text: "Overview"},
			ui.ActionButton{
				Label:   "Refresh overview",
				Action:  overviewAction.HTMX(),
				Variant: components.ButtonSecondary,
			},
		),
		components.Grid{Columns: 4, Gap: "1rem", Children: []html.Component{
			shared.Stat{Label: "Members", Value: strconv.Itoa(len(model.Members))},
			shared.Stat{Label: "Active accounts", Value: strconv.Itoa(active)},
			shared.Stat{Label: "Administrators", Value: strconv.Itoa(admins)},
			shared.Stat{Label: "Saved changes", Value: strconv.Itoa(model.Changes)},
		}},
		components.Paragraph("Overview is a snapshot. Refresh it independently after editing members."),
	)
}

func workspace(model Model) html.Component {
	directory := model.directory()
	filter, pagination := directoryNavigation(model, directory)
	var results html.Component = memberTable(directory.Members)
	if directory.Total == 0 {
		results = components.EmptyState{
			Title: "No matching members", Description: "Try another name or role.",
		}
	}
	return components.Column(
		components.Card{
			Header: components.Heading{Level: 2, Text: "Member directory"},
			Body: components.Column(
				filter,
				components.Paragraph(fmt.Sprintf("%d matching members", directory.Total)),
				results,
				pagination,
			),
		},
		html.Optional{Show: model.Notice != "", Body: components.Paragraph(model.Notice)},
		editor(model),
	)
}

func directoryNavigation(model Model, directory directoryPage) (ui.Form, components.Pagination) {
	action := browseAction.HTMX()
	action.PushURL = "true"
	attrs := html.Attrs{html.A("hx-sync", "closest main:replace")}
	filter := ui.Form{
		Action: action,
		Body: components.Row(
			components.Input{
				Name:        "q",
				Label:       "Search members",
				Value:       model.Query,
				Placeholder: "Name or email",
			},
			components.Select{
				Name:        "role",
				Label:       "Filter by role",
				Value:       model.Role,
				Placeholder: "All roles",
				Options:     roleOptions(),
			},
			components.Button{Type: components.ButtonSubmit, Label: "Apply filters"},
		),
		Attrs: attrs,
	}
	query := url.Values{"q": {model.Query}, "role": {model.Role}}
	return filter, components.Pagination{
		Page:       directory.Page,
		PageSize:   pageSize,
		TotalItems: directory.Total,
		BaseURL:    action.URL + "?" + query.Encode(),
		Target:     action.Target,
		Swap:       action.Swap,
		PushURL:    action.PushURL == "true",
		Attrs:      attrs,
	}
}

func roleOptions() []components.Option {
	options := make([]components.Option, len(roles))
	for index, role := range roles {
		options[index] = components.Option{Value: role, Label: role}
	}
	return options
}

func memberTable(members []Member) html.Component {
	return components.Table{
		Caption: "Members",
		Header:  components.HeaderRow{"Name", "Email", "Role", "Status", "Actions"},
		Body: components.Map(members, func(member Member) html.Component {
			status := "Inactive"
			if member.Active {
				status = "Active"
			}
			action := editAction.HTMX()
			action.URL += "?id=" + strconv.Itoa(member.ID)
			return components.TableRow{Cells: []html.Component{
				components.RowHeader(member.Name),
				components.TextCell(member.Email),
				components.TextCell(member.Role),
				components.TextCell(status),
				components.TableCell{Body: components.Link{
					Label:  "Edit " + member.Name,
					URL:    action.URL,
					Action: action,
				}},
			}}
		}),
	}
}

func editor(model Model) html.Component {
	if model.Form.Value.ID == 0 {
		return components.Card{
			Header: components.Heading{Level: 2, Text: "Member editor"},
			Body:   components.Paragraph("Choose Edit beside a member to update their profile."),
		}
	}
	fields := ui.Fields(model.Form.Errors)
	value := model.Form.Value
	return components.Card{
		Header: components.Heading{Level: 2, Text: "Edit member"},
		Body: ui.Form{
			Action: saveAction.HTMX(),
			Body: components.FormLayout(
				components.Input{Name: "id", Type: "hidden", Value: strconv.Itoa(value.ID)},
				fields.Input(components.Input{
					Name:     "name",
					Label:    "Member name",
					Value:    value.Name,
					Required: true,
				}),
				fields.Select(components.Select{
					Name:     "role",
					Label:    "Member role",
					Value:    value.Role,
					Options:  roleOptions(),
					Required: true,
				}),
				components.Button{Type: components.ButtonSubmit, Label: "Save member"},
			),
		},
	}
}
