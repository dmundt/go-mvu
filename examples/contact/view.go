package main

import (
	"context"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/validation"
	"github.com/dmundt/go-mvu/web"
)

const contactRegion ui.Region = "contact-form"

type ContactView struct{ Document ui.Document }

func (view ContactView) Page(
	_ context.Context,
	model ContactModel,
	request web.RequestContext,
) (html.Page, error) {
	return view.Document.Page(request, "Contact", components.AppShell{
		Header: components.Heading{Level: 1, Text: "Contact us"},
		Main: components.Card{
			Header: components.Heading{Level: 2, Text: "Send a request"},
			Body: contactRegion.Wrap(ContactForm{
				Form: model.Form, Submitted: model.Submitted,
			}),
		},
	}), nil
}

func (ContactView) Fragment(
	_ context.Context,
	name string,
	model ContactModel,
	_ web.RequestContext,
) (html.Fragment, error) {
	return contactRegion.Select(name, ContactForm{
		Form: model.Form, Submitted: model.Submitted,
	})
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
		fields.Text("name", "Your name", details.Name, true),
		fields.Email("email", "Email address", details.Email, true),
		fields.Text("company", "Company (optional)", details.Company, false),
		fields.Select(components.Select{
			Name:  "topic",
			Label: "Topic",
			Value: details.Topic,
			Options: []components.Option{
				{Value: "", Label: "Choose one"},
				{Value: "support", Label: "Support"},
				{Value: "sales", Label: "Sales"},
				{Value: "other", Label: "Other"},
			},
			Required: true,
		}),
		fields.Textarea(components.Textarea{
			Name:     "message",
			Label:    "How can we help?",
			Value:    details.Message,
			Hint:     "20 to 1000 characters.",
			Required: true,
		}),
		fields.Checkbox(components.Checkbox{
			Name:     "agreement",
			Value:    "yes",
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
