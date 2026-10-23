package ui

import (
	"testing"

	"github.com/dmundt/go-mvu/components"
)

func TestFieldsClearStaleErrorsWithoutChangingControlProperties(t *testing.T) {
	fields := Fields(nil)
	input := fields.Input(components.Input{Name: "name", Label: "Name", Value: "Ada", Required: true, Error: "stale"})
	selectInput := fields.Select(components.Select{Name: "topic", Value: "support", Placeholder: "Choose", Error: "stale"})
	textarea := fields.Textarea(components.Textarea{Name: "message", Value: "Hello", Hint: "Hint", Error: "stale"})
	checkbox := fields.Checkbox(components.Checkbox{Name: "agreement", Checked: true, Value: "yes", Error: "stale"})
	if input.Error != "" || input.Value != "Ada" || !input.Required || input.Label != "Name" {
		t.Fatalf("input = %+v", input)
	}
	if selectInput.Error != "" || selectInput.Value != "support" || selectInput.Placeholder != "Choose" {
		t.Fatalf("select = %+v", selectInput)
	}
	if textarea.Error != "" || textarea.Value != "Hello" || textarea.Hint != "Hint" {
		t.Fatalf("textarea = %+v", textarea)
	}
	if checkbox.Error != "" || !checkbox.Checked || checkbox.Value != "yes" {
		t.Fatalf("checkbox = %+v", checkbox)
	}
}
