package main

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/validation"
	"github.com/dmundt/go-mvu/web"
)

const contactRegion ui.Region = "contact-form"

func contactView(document ui.Document) web.ViewFunctions[ContactModel] {
	return web.ViewFunctions[ContactModel]{
		PageFunc: func(
			_ context.Context, model ContactModel, request web.RequestContext,
		) (html.Page, error) {
			return document.Page(request, "Contact", components.AppShell{
				Header: components.Heading{Level: 1, Text: "Contact us"},
				Main: components.Card{
					Header: components.Heading{Level: 2, Text: "Send a request"},
					Body:   contactRegion.Wrap(contactBody(model)),
				},
			}), nil
		},
		FragmentFunc: func(
			_ context.Context, name string, model ContactModel, _ web.RequestContext,
		) (html.Fragment, error) {
			return contactRegion.Select(name, contactBody(model))
		},
	}
}

func contactBody(model ContactModel) html.Component {
	if model.Submitted {
		return components.Column(
			components.Heading{Level: 2, Text: "Request received"},
			components.Paragraph("Thanks. We'll get back to you soon."),
		)
	}
	return ui.Form{
		Action: submitContact.HTMX(),
		Attrs:  html.Attrs{html.A("class", "contact-form")},
		Body:   components.FormLayout(contactFields(model.Form)...),
	}
}

func contactFields(form validation.Form[ContactDetails]) []html.Component {
	details := form.Value
	fields := ui.Fields(form.Errors)
	return []html.Component{
		fields.Input(components.Input{
			Name:     nameField,
			Label:    "Your name",
			Value:    details.Name,
			Required: true,
		}),
		fields.Input(components.Input{
			Name:     emailField,
			Label:    "Email address",
			Type:     "email",
			Value:    details.Email,
			Required: true,
		}),
		fields.Input(components.Input{
			Name:  companyField,
			Label: "Company (optional)",
			Value: details.Company,
		}),
		fields.Select(components.Select{
			Name:        topicField,
			Label:       "Topic",
			Value:       details.Topic,
			Placeholder: "Choose one",
			Options:     contactTopics(),
			Required:    true,
		}),
		fields.Textarea(components.Textarea{
			Name:     messageField,
			Label:    "How can we help?",
			Value:    details.Message,
			Hint:     fmt.Sprintf("%d to %d characters.", messageMinLength, messageMaxLength),
			Required: true,
		}),
		fields.Checkbox(components.Checkbox{
			Name:     agreementField,
			Value:    agreementValue,
			Label:    "You may reply to my request.",
			Checked:  details.Agreement,
			Required: true,
		}),
		components.Button{
			Type:  components.ButtonSubmit,
			Label: "Send request",
		},
	}
}
