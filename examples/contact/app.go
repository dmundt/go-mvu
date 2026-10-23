package main

import (
	"context"

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
