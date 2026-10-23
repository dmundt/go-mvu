package ui

import (
	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/validation"
)

// Fields attaches errors by explicit control name. Values and validation rules
// remain application-owned; no reflection or request binding is performed.
type Fields validation.Errors

func (fields Fields) Text(name, label, value string, required bool) components.Input {
	return fields.Input(components.Input{
		Name: name, Label: label, Value: value, Type: "text", Required: required,
	})
}

func (fields Fields) Email(name, label, value string, required bool) components.Input {
	return fields.Input(components.Input{
		Name: name, Label: label, Value: value, Type: "email", Required: required,
	})
}

func (fields Fields) Input(input components.Input) components.Input {
	input.Error = validation.Errors(fields).First(input.Name)
	return input
}

func (fields Fields) Select(selectField components.Select) components.Select {
	selectField.Error = validation.Errors(fields).First(selectField.Name)
	return selectField
}

func (fields Fields) Textarea(textarea components.Textarea) components.Textarea {
	textarea.Error = validation.Errors(fields).First(textarea.Name)
	return textarea
}

func (fields Fields) Checkbox(checkbox components.Checkbox) components.Checkbox {
	checkbox.Error = validation.Errors(fields).First(checkbox.Name)
	return checkbox
}
