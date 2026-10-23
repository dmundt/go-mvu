package main

import (
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/validation"
	"github.com/dmundt/go-mvu/web"
)

var submitContact = web.Action[ContactMessage]{
	Method: htmx.Post,
	Path:   "/contact",
	Decode: decodeContact,
	Result: web.Result{Mode: web.RenderAuto, Fragment: string(contactRegion)},
}

func decodeContact(request *http.Request) (ContactMessage, error) {
	form, err := validation.Bind(request, decodeContactDetails)
	if err != nil {
		return nil, err
	}
	return SubmitContact{Form: form}, nil
}

func decodeContactDetails(request *http.Request) (ContactDetails, validation.Errors, error) {
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
		errs.Add(nameField, fmt.Sprintf("Enter at least %d characters.", nameMinLength))
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
