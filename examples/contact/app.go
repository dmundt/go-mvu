package main

import (
	"context"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/validation"
)

type ContactDetails struct {
	Name      string
	Email     string
	Company   string
	Topic     string
	Message   string
	Agreement bool
}

type ContactModel struct {
	Form      validation.Form[ContactDetails]
	Submitted bool
}

type ContactMessage interface {
	mvu.Message
	isContactMessage()
}

type SubmitContact struct {
	Form validation.Form[ContactDetails]
}

func (SubmitContact) MessageName() string { return "contact.submit" }
func (SubmitContact) isContactMessage()   {}

type ContactApp struct{}

func (ContactApp) Init(context.Context) (ContactModel, []mvu.Command[ContactMessage], error) {
	return ContactModel{}, nil, nil
}

func (ContactApp) Update(
	_ context.Context,
	model ContactModel,
	message ContactMessage,
) (ContactModel, []mvu.Command[ContactMessage], error) {
	switch message := message.(type) {
	case SubmitContact:
		model.Form = message.Form
		model.Submitted = message.Form.Valid
	}
	return model, nil, nil
}

type ContactDecoder struct{}

func (ContactDecoder) Decode(
	request *http.Request,
) (ContactDetails, validation.Errors, error) {
	if err := request.ParseForm(); err != nil {
		return ContactDetails{}, nil, err
	}
	values := request.PostForm
	details := ContactDetails{
		Name:      strings.TrimSpace(values.Get("name")),
		Email:     strings.TrimSpace(values.Get("email")),
		Company:   strings.TrimSpace(values.Get("company")),
		Topic:     values.Get("topic"),
		Message:   strings.TrimSpace(values.Get("message")),
		Agreement: values.Get("agreement") == "yes",
	}
	errs := validation.Errors{}
	validateContact(details, errs)
	return details, errs, nil
}

func validateContact(details ContactDetails, errs validation.Errors) {
	if utf8.RuneCountInString(details.Name) < 2 {
		errs.Add("name", "Enter at least two characters.")
	}
	if !validEmail(details.Email) {
		errs.Add("email", "Enter a valid email address.")
	}
	switch details.Topic {
	case "support", "sales", "other":
	default:
		errs.Add("topic", "Choose a topic.")
	}
	messageLength := utf8.RuneCountInString(details.Message)
	if messageLength < 20 {
		errs.Add("message", "Enter at least 20 characters.")
	} else if messageLength > 1000 {
		errs.Add("message", "Keep your message under 1000 characters.")
	}
	if !details.Agreement {
		errs.Add("agreement", "Confirm we may reply to your request.")
	}
}

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return false
	}
	_, domain, ok := strings.Cut(value, "@")
	return ok && strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
}

func decodeContact(request *http.Request) (ContactMessage, error) {
	form, err := validation.Bind(request, ContactDecoder{})
	if err != nil {
		return nil, err
	}
	return SubmitContact{Form: form}, nil
}
