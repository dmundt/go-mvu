package main

import (
	"slices"

	"github.com/dmundt/go-mvu/components"
)

const (
	nameField        = "name"
	emailField       = "email"
	companyField     = "company"
	topicField       = "topic"
	messageField     = "message"
	agreementField   = "agreement"
	agreementValue   = "yes"
	nameMinLength    = 2
	messageMinLength = 20
	messageMaxLength = 1000
)

// contactTopics is shared by the select and server validation. The empty
// placeholder is deliberately excluded from accepted values.
func contactTopics() []components.Option {
	return []components.Option{
		{Value: "support", Label: "Support"},
		{Value: "sales", Label: "Sales"},
		{Value: "other", Label: "Other"},
	}
}

func validContactTopic(value string) bool {
	return slices.ContainsFunc(contactTopics(), func(option components.Option) bool {
		return option.Value == value
	})
}
