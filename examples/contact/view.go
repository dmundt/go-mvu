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

type ContactView struct {
	ui.RegionView[ContactModel]
	Document ui.Document
}

func contactContent(
	_ context.Context, model ContactModel, _ web.RequestContext,
) (html.Component, error) {
	return ContactForm{Form: model.Form, Submitted: model.Submitted}, nil
}

func (view ContactView) Page(
	ctx context.Context,
	model ContactModel,
	request web.RequestContext,
) (html.Page, error) {
	content, err := view.Render(ctx, model, request)
	if err != nil {
		return nil, err
	}
	return view.Document.Page(request, "Contact", components.AppShell{
		Header: components.Heading{Level: 1, Text: "Contact us"},
		Main: components.Card{
			Header: components.Heading{Level: 2, Text: "Send a request"},
			Body:   content,
		},
	}), nil
}

type ContactForm struct {
	Form      validation.Form[ContactDetails]
	Submitted bool
}

func (view ContactForm) RenderHTML(ctx context.Context, out *html.Context) error {
	if view.Submitted {
		out.Component(ctx, components.Column(
			components.Heading{Level: 2, Text: "Request received"},
			components.Paragraph("Thanks. We'll get back to you soon."),
		))
		return out.Err()
	}

	out.Component(ctx, ui.Form{
		Action: submitContact.HTMX(),
		Attrs:  html.Attrs{html.A("class", "contact-form")},
		Body:   components.FormLayout(contactFields(view.Form)...),
	})
	return out.Err()
}

func contactFields(form validation.Form[ContactDetails]) []html.Component {
	details := form.Value
	fields := ui.Fields(form.Errors)
	return []html.Component{
		fields.Text(nameField, "Your name", details.Name, true),
		fields.Email(emailField, "Email address", details.Email, true),
		fields.Text(companyField, "Company (optional)", details.Company, false),
		fields.Select(components.Select{
			Name:     topicField,
			Label:    "Topic",
			Value:    details.Topic,
			Options:  append([]components.Option{{Label: "Choose one"}}, contactTopics()...),
			Required: true,
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
