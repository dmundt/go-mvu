package main

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

type View struct{ Document components.PagePolicy }

func (view View) Page(
	_ context.Context, model Model, request web.RequestContext,
) (html.Page, error) {
	return view.Document.Page(request, "Operations dashboard", components.Shell{
		Header: components.Row(
			components.Heading{Level: 1, Text: "Northstar / Operations"},
			components.SecureThemeSwitcher{
				Mode:   request.Mode,
				Return: "/admin/view",
			},
		),
		Sidebar: components.Navigation{Label: "Administration", Items: []components.NavItem{
			{Label: "Overview", URL: "#admin-overview"},
			{Label: "Members", URL: "#admin-workspace"},
			{Label: "About this demo", URL: "#about-demo"},
		}},
		Main: components.Column(
			overviewRegion.Wrap(overview(model)),
			saveStatus(model, false),
			workspaceRegion.Wrap(workspace(model, true)),
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
		return workspaceRegion.Wrap(workspace(model, false)), nil
	case string(overviewRegion):
		return overviewRegion.Wrap(overview(model)), nil
	case string(editorRegion):
		return editorFragment{Model: model}, nil
	case "member-saved":
		return savedFragment{Model: model}, nil
	default:
		return nil, web.ErrUnknownFragment
	}
}

type editorFragment struct {
	html.FragmentMarker
	Model Model
}

func (fragment editorFragment) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, editor(fragment.Model))
	return out.Err()
}

type savedFragment struct {
	html.FragmentMarker
	Model Model
}

func (fragment savedFragment) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, workspaceRegion.Wrap(workspace(fragment.Model, false)))
	out.Component(ctx, saveStatus(fragment.Model, true))
	return out.Err()
}

func saveStatus(model Model, oob bool) html.Component {
	attrs := html.Attrs{html.A("role", "status"), html.A("aria-live", "polite")}
	if oob {
		attrs = append(attrs, html.A("hx-swap-oob", "innerHTML"))
	}
	return components.Section{
		ID: "save-status", Attrs: attrs,
		Children: []html.Component{
			html.Optional{Show: model.Notice != "", Body: components.Paragraph(model.Notice)},
		},
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
			components.ActionButton{
				Label:   "Refresh overview",
				Action:  overviewAction.HTMX(),
				Variant: components.ButtonSecondary,
			},
		),
		components.Grid{Columns: 4, Gap: "1rem", Children: []html.Component{
			components.Stat{Label: "Members", Value: strconv.Itoa(len(model.Members))},
			components.Stat{Label: "Active accounts", Value: strconv.Itoa(active)},
			components.Stat{Label: "Administrators", Value: strconv.Itoa(admins)},
			components.Stat{Label: "Saved changes", Value: strconv.Itoa(model.Changes)},
		}},
		components.Paragraph("Overview is a snapshot. Refresh it independently after editing members."),
	)
}

func workspace(model Model, inlineEditor bool) html.Component {
	directory := model.directory()
	filter, pagination := directoryNavigation(model, directory)
	var results html.Component = memberTable(directory.Members)
	if directory.Total == 0 {
		results = components.EmptyState{
			Title: "No matching members", Description: "Try another name or role.",
		}
	}
	body := components.Column(
		components.Card{
			Header: components.Heading{Level: 2, Text: "Member directory"},
			Body: components.Column(
				filter,
				components.Paragraph(fmt.Sprintf("%d matching members", directory.Total)),
				results,
				pagination,
			),
		},
		memberEditorShell(),
		html.Optional{Show: inlineEditor && model.Form.Value.ID != 0, Body: editor(model)},
	)
	// All requests that can replace this workspace share a cancellation scope.
	body.Attrs = filter.Attrs
	return body
}

func directoryNavigation(model Model, directory directoryPage) (components.ActionForm, components.Pagination) {
	action := browseAction.HTMX()
	action.PushURL = "true"
	attrs := html.Attrs{html.A("hx-sync", "closest main:replace")}
	var filterAttrs html.Attrs
	if model.Notice != "" {
		filterAttrs = html.Attrs{html.Flag("autofocus")}
	}
	filter := components.ActionForm{
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
			components.Button{
				Type: components.ButtonSubmit, Label: "Apply filters",
				Attrs: filterAttrs,
			},
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

func memberEditorShell() html.Component {
	return components.Section{
		ID: editorPopover,
		Attrs: html.Attrs{
			html.A("class", "dialog"), html.A("popover", "auto"),
			html.A("role", "dialog"), html.A("aria-labelledby", "member-editor-title"),
		},
		Children: []html.Component{components.Column(
			components.Heading{
				Level: 2, Text: "Quick edit member",
				Attrs: html.Attrs{html.A("id", "member-editor-title")},
			},
			components.Paragraph(
				"Non-modal editor. Other page controls remain available. "+
					"Without HTMX, use the full-page Edit link in the directory.",
			),
			components.Button{
				Label: "Cancel quick edit", Action: cancelAction.HTMX(),
				Attrs: html.Attrs{
					html.A("popovertarget", editorPopover),
					html.A("popovertargetaction", "hide"),
				},
			},
			editorRegion.Wrap(components.Paragraph("Loading member. Use Edit for a full page.")),
		)},
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
				components.TableCell{Body: components.Column(
					components.Link{Label: "Edit " + member.Name, URL: action.URL},
					components.Button{
						Label: "Quick edit " + member.Name, Action: action,
						Attrs: html.Attrs{
							html.A("popovertarget", editorPopover),
							html.A("popovertargetaction", "show"),
						},
					},
				)},
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
	fields := components.Fields(model.Form.Errors)
	value := model.Form.Value
	action := saveAction.HTMX()
	action.DisabledElt = "find button"
	return components.Card{
		Header: components.Heading{Level: 2, Text: "Edit member"},
		Body: components.ActionForm{
			Action: action,
			Body: components.FormLayout(
				components.Input{Name: "id", Type: "hidden", Value: strconv.Itoa(value.ID)},
				fields.Input(components.Input{
					Name:     "name",
					Label:    "Member name",
					Value:    value.Name,
					Required: true,
					Attrs:    html.Attrs{html.Flag("autofocus")},
				}),
				fields.Select(components.Select{
					Name:     "role",
					Label:    "Member role",
					Value:    value.Role,
					Options:  roleOptions(),
					Required: true,
				}),
				components.Button{Type: components.ButtonSubmit, Label: "Save member"},
				components.Link{Label: "Cancel editing", URL: cancelAction.Path},
			),
		},
	}
}
