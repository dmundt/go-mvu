package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/ui"
	"github.com/dmundt/go-mvu/validation"
	"github.com/dmundt/go-mvu/web"
)

func TestContactFunctionsShareSubmittedContentAndRoot(t *testing.T) {
	view := contactView(ui.Document{})
	model := ContactModel{Submitted: true}
	page, err := view.Page(context.Background(), model, web.RequestContext{})
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := view.Fragment(
		context.Background(), string(contactRegion), model, web.RequestContext{},
	)
	if err != nil {
		t.Fatal(err)
	}
	var pageOutput, fragmentOutput strings.Builder
	if err := html.Render(context.Background(), &pageOutput, page); err != nil {
		t.Fatal(err)
	}
	if err := html.Render(context.Background(), &fragmentOutput, fragment); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pageOutput.String(), fragmentOutput.String()) ||
		!strings.HasPrefix(fragmentOutput.String(), `<section id="contact-form">`) ||
		strings.Count(pageOutput.String(), `id="contact-form"`) != 1 {
		t.Fatalf("page/fragment roots differ: %s", fragmentOutput.String())
	}
	if _, err := view.Fragment(
		context.Background(), "unknown", model, web.RequestContext{},
	); !errors.Is(err, web.ErrUnknownFragment) {
		t.Fatalf("unknown fragment error = %v", err)
	}
}

func TestContactNameErrorUsesConfiguredLimit(t *testing.T) {
	errs := validation.Errors{}
	validateContact(ContactDetails{}, errs)
	want := fmt.Sprintf("Enter at least %d characters.", nameMinLength)
	if got := errs.First(nameField); got != want {
		t.Fatalf("name error = %q, want %q", got, want)
	}
}
