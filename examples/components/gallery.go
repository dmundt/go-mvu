package main

import (
	"context"
	"net/url"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/examples/internal/modepicker"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
)

type tab struct{ ID, Label, Text string }

var tabs = []tab{
	{
		ID:    "overview",
		Label: "Overview",
		Text: "Tabs are ordinary links. The server renders the selected panel, " +
			"so navigation works without JavaScript.",
	},
	{
		ID:    "activity",
		Label: "Activity",
		Text: "Three deployments finished today. " +
			"Tab state lives in the URL and survives reloads.",
	},
	{
		ID:    "settings",
		Label: "Settings",
		Text: "Pass HTMX: true to swap the tab set in place " +
			"while keeping full-page fallback links.",
	},
}

type gallery struct {
	Mode theme.Mode
	Tab  tab
}

func (view gallery) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, components.Main{Children: []html.Component{
		components.Heading{Level: 1, Text: "Component gallery"},
		modepicker.Form{
			Mode:   view.Mode,
			Hidden: []modepicker.Hidden{{Name: "tab", Value: view.Tab.ID}},
		},
		view.content(),
	}})
	return out.Err()
}

func (view gallery) content() html.Component {
	cards := components.Grid{Columns: 2, Gap: "1.5rem", Children: []html.Component{
		view.tabsCard(),
		accordionCard(),
		overlaysCard(),
		avatarsCard(),
		feedbackCard(),
		formCard(),
		stepsCard(),
		emptyCard(),
		mixedCard(),
	}}
	return components.Stack{
		Gap:      "1.5rem",
		Children: []html.Component{stats(), cards},
	}
}

// url links to tab id while keeping the selected mode.
func (view gallery) url(id string) string {
	query := url.Values{
		"mode": {view.Mode.String()},
		"tab":  {id},
	}
	return "?" + query.Encode()
}

func stats() html.Component {
	return components.Grid{Columns: 4, Gap: "1rem", Children: []html.Component{
		components.Stat{
			Label: "Revenue", Value: "$48.2k",
			Delta: "+12.4% this month", Trend: components.TrendUp,
		},
		components.Stat{Label: "Active users", Value: "2,431", Delta: "+3.1%", Trend: components.TrendUp},
		components.Stat{Label: "Churn", Value: "1.8%", Delta: "-0.4%", Trend: components.TrendDown},
		components.Stat{Label: "Uptime", Value: "99.98%", Delta: "No change"},
	}}
}

func (view gallery) tabsCard() html.Component {
	items := make([]components.Tab, len(tabs))
	for index, item := range tabs {
		items[index] = components.Tab{ID: item.ID, Label: item.Label, URL: view.url(item.ID)}
	}
	return card("Tabs", components.Tabs{
		ID:     "gallery-tabs",
		Label:  "Gallery sections",
		Items:  items,
		Active: view.Tab.ID,
		Panel:  components.Paragraph(view.Tab.Text),
	})
}

func accordionCard() html.Component {
	return card("Accordion", components.Accordion{
		ID:        "faq",
		Exclusive: true,
		Items: []components.AccordionItem{
			{
				Title: "Does it need JavaScript?",
				Body:  components.Paragraph("No. It uses native details and summary elements."),
				Open:  true,
			},
			{
				Title: "Can sections stay open together?",
				Body:  components.Paragraph("Yes. Leave Exclusive false to allow several open sections."),
			},
			{
				Title: "Who styles it?",
				Body:  components.Paragraph("The selected mode. Markup stays theme neutral."),
			},
		},
	})
}

func overlaysCard() html.Component {
	dialog := components.Dialog{
		ID:             "invite-dialog",
		Title:          "Invite teammate",
		TriggerLabel:   "Open dialog",
		TriggerVariant: components.ButtonPrimary,
		Body: components.Input{
			ID:          "invite-email",
			Name:        "email",
			Type:        "email",
			Label:       "Email",
			Placeholder: "ada@example.test",
		},
		Footer: components.Sequence{
			components.Button{Label: "Cancel", Variant: components.ButtonSecondary},
			components.Button{Label: "Send invite", Variant: components.ButtonPrimary},
		},
	}
	dropdown := components.Dropdown{Label: "Actions", Items: []components.MenuItem{
		{Label: "Edit profile", URL: "#profile"},
		{Label: "Duplicate", URL: "#duplicate"},
		{Label: "Email support", URL: "mailto:support@example.test"},
	}}
	tooltip := components.Tooltip{
		ID:      "save-tip",
		Label:   "Show help",
		Content: "Tooltips describe their trigger.",
	}
	return card("Overlays",
		components.Paragraph("Dialogs use the native popover attribute; dropdowns use details."),
		inline(dialog, dropdown),
		inline(tooltip),
	)
}

func avatarsCard() html.Component {
	return card("Avatars and keys",
		inline(
			components.Avatar{Name: "Ada Lovelace", Size: components.AvatarSmall},
			components.Avatar{Name: "Grace Hopper"},
			components.Avatar{Name: "Alan Turing", Size: components.AvatarLarge},
		),
		components.Separator{Label: "Shortcuts"},
		inline(
			components.Kbd{Keys: []string{"Ctrl", "K"}},
			components.Text("Command palette"),
		),
	)
}

func feedbackCard() html.Component {
	return card("Feedback",
		components.Progress{ID: "upload-progress", Label: "Uploading report", Value: 64, Max: 100},
		components.Progress{ID: "sync-progress", Label: "Syncing"},
		inline(
			components.Spinner{ID: "loading", Label: "Loading results"},
			components.Text("Loading results"),
		),
		components.Skeleton{Lines: 3},
	)
}

func formCard() html.Component {
	return card("Form controls", components.FormLayout(
		components.Switch{Name: "notifications", Label: "Email notifications", Checked: true},
		components.Checkbox{
			Name:     "terms",
			Label:    "I accept the terms",
			Required: true,
			Error:    "Accept the terms to continue.",
		},
		components.RadioGroup{
			Name:   "plan",
			Legend: "Plan",
			Value:  "team",
			Options: []components.Option{
				{Value: "free", Label: "Free"},
				{Value: "team", Label: "Team"},
				{Value: "enterprise", Label: "Enterprise", Disabled: true},
			},
		},
		components.Textarea{
			Name:        "message",
			Label:       "Message",
			Placeholder: "Write a short note",
			Hint:        "Markdown is not supported.",
			Rows:        3,
		},
	))
}

func stepsCard() html.Component {
	return card("Steps", components.Steps{
		Label:   "Checkout progress",
		Items:   []string{"Account", "Shipping", "Payment", "Review"},
		Current: 2,
	})
}

func emptyCard() html.Component {
	return card("Empty state", components.EmptyState{
		Title:       "No projects yet",
		Description: "Create a project to start tracking work.",
		Action:      components.Button{Label: "New project", Variant: components.ButtonPrimary},
	})
}

func card(title string, body ...html.Component) html.Component {
	return components.Card{
		Header: components.Text(title),
		Body:   components.Stack{Children: body},
	}
}

func inline(children ...html.Component) html.Component {
	return components.Inline{Children: children}
}
