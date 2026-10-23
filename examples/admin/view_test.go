package main

import (
	"context"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func TestDirectoryNavigationSharesPolicyAndPreservesQuery(t *testing.T) {
	model := Model{Query: "Ada & Grace + Example", Role: roleAdmin}
	filter, pagination := directoryNavigation(model, directoryPage{Total: 6, Page: 1})
	if filter.Action.Target != pagination.Target || filter.Action.Swap != pagination.Swap ||
		filter.Action.PushURL != "true" || !pagination.PushURL ||
		!reflect.DeepEqual(filter.Attrs, pagination.Attrs) ||
		!reflect.DeepEqual(filter.Attrs, html.Attrs{html.A("hx-sync", "closest main:replace")}) {
		t.Fatalf("navigation policies differ: filter=%+v pagination=%+v", filter, pagination)
	}
	if pagination.URL != nil {
		t.Fatal("pagination should use BaseURL, not a callback")
	}
	base, err := url.Parse(pagination.BaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if base.Path != browseAction.Path || base.Query().Get("q") != model.Query ||
		base.Query().Get("role") != model.Role {
		t.Fatalf("base URL = %s", base)
	}
	var output strings.Builder
	if err := html.Render(context.Background(), &output, pagination); err != nil {
		t.Fatal(err)
	}
	query := base.Query()
	query.Set("page", "2")
	want := browseAction.Path + "?" + strings.ReplaceAll(query.Encode(), "&", "&amp;")
	if !strings.Contains(output.String(), `href="`+want+`"`) ||
		!strings.Contains(output.String(), `hx-get="`+want+`"`) {
		t.Fatalf("pagination lost filters: %s", output.String())
	}
	if editAction.HTMX().PushURL != "" || saveAction.HTMX().PushURL != "" ||
		overviewAction.HTMX().PushURL != "" {
		t.Fatal("directory navigation policy leaked to another action")
	}
}

func TestRoleChoicesMatchValidationAndSeedData(t *testing.T) {
	options := roleOptions()
	if len(options) != len(roles) {
		t.Fatalf("options = %+v", options)
	}
	for index, role := range roles {
		if !validRole(role) || options[index].Value != role || options[index].Label != role {
			t.Fatalf("role %q disagrees with options or validation", role)
		}
	}
	if validRole("") || validRole("Owner") {
		t.Fatal("unexpected role accepted")
	}
	model, _, err := (App{}).Init(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range model.Members {
		if !validRole(member.Role) {
			t.Fatalf("invalid seed role: %q", member.Role)
		}
	}
}
