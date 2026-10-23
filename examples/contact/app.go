package main

import (
	"context"
	"fmt"
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
		Name:      strings.TrimSpace(values.Get(nameField)),
		Email:     strings.TrimSpace(values.Get(emailField)),
		Company:   strings.TrimSpace(values.Get(companyField)),
		Topic:     values.Get(topicField),
		Message:   strings.TrimSpace(values.Get(messageField)),
		Agreement: values.Get(agreementField) == agreementValue,
	}
	errs := validation.Errors{}
	validateContact(details, errs)
	return details, errs, nil
}

func validateContact(details ContactDetails, errs validation.Errors) {
	if utf8.RuneCountInString(details.Name) < nameMinLength {
		errs.Add(nameField, "Enter at least two characters.")
	}
	if !validEmail(details.Email) {
		errs.Add(emailField, "Enter a valid email address.")
	}
	if !validContactTopic(details.Topic) {
		errs.Add(topicField, "Choose a topic.")
	}
	messageLength := utf8.RuneCountInString(details.Message)
	if messageLength < messageMinLength {
		errs.Add(messageField, fmt.Sprintf("Enter at least %d characters.", messageMinLength))
	} else if messageLength > messageMaxLength {
		errs.Add(messageField, fmt.Sprintf("Keep your message under %d characters.", messageMaxLength))
	}
	if !details.Agreement {
		errs.Add(agreementField, "Confirm we may reply to your request.")
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
