package components

import (
	"github.com/dmundt/go-mvu/web"
)

// Fields attaches errors by explicit control name. Values and validation rules
// remain application-owned; no reflection or request binding is performed.
// All methods replace the supplied Error, clearing it when no error matches.
type Fields web.FieldErrors

func (fields Fields) Text(name, label, value string, required bool) Input {
	return fields.Input(Input{
		Name: name, Label: label, Value: value, Type: "text", Required: required,
	})
}

func (fields Fields) Email(name, label, value string, required bool) Input {
	return fields.Input(Input{
		Name: name, Label: label, Value: value, Type: "email", Required: required,
	})
}

func (fields Fields) Input(input Input) Input {
	input.Error = web.FieldErrors(fields).First(input.Name)
	return input
}

func (fields Fields) Select(selectField Select) Select {
	selectField.Error = web.FieldErrors(fields).First(selectField.Name)
	return selectField
}

func (fields Fields) Textarea(textarea Textarea) Textarea {
	textarea.Error = web.FieldErrors(fields).First(textarea.Name)
	return textarea
}

func (fields Fields) Checkbox(checkbox Checkbox) Checkbox {
	checkbox.Error = web.FieldErrors(fields).First(checkbox.Name)
	return checkbox
}
