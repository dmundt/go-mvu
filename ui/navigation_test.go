package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/theme"
)

func TestActionLinkSharesEscapedURLAndPresentation(t *testing.T) {
	output := render(t, ActionLink{
		Action: htmx.Action{
			Method: htmx.Get, URL: "/edit?id=1&q=%3Cname%3E&role=Admin",
			Target: "#members", Swap: htmx.OuterHTML, PushURL: "true",
		},
		Label: "Edit <member>", Variant: components.ButtonSecondary,
		Attrs: html.Attrs{html.A("aria-label", "Edit <profile>")},
	})
	for _, want := range []string{
		`href="/edit?id=1&amp;q=%3Cname%3E&amp;role=Admin"`,
		`hx-get="/edit?id=1&amp;q=%3Cname%3E&amp;role=Admin"`,
		`hx-target="#members"`, `hx-swap="outerHTML"`, `hx-push-url="true"`,
		`Edit &lt;member&gt;`, `button--secondary`, `aria-label="Edit &lt;profile&gt;"`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q: %s", want, output)
		}
	}
}

func TestActionLinkRejectsInvalidActionsBeforeOutput(t *testing.T) {
	for _, test := range []struct {
		action htmx.Action
		err    error
	}{
		{htmx.Action{}, ErrInvalidLink},
		{htmx.Action{Method: htmx.Post, URL: "/save"}, ErrInvalidLink},
		{htmx.Action{Method: htmx.Get, URL: "javascript:alert(1)"}, htmx.ErrInvalidAction},
		{htmx.Action{Method: htmx.Get, URL: "/edit", Swap: "invalid"}, htmx.ErrInvalidAction},
	} {
		var output strings.Builder
		err := html.Render(context.Background(), &output, ActionLink{Action: test.action})
		if !errors.Is(err, test.err) || output.Len() != 0 {
			t.Fatalf("action=%+v error=%v output=%q", test.action, err, output.String())
		}
	}
}

func TestThemeSwitcherRequiresCSRFBeforeOutput(t *testing.T) {
	var output strings.Builder
	err := html.Render(context.Background(), &output, ThemeSwitcher{Mode: theme.Dark, Return: "/"})
	if !errors.Is(err, middleware.ErrCSRFTokenUnset) || output.Len() != 0 {
		t.Fatalf("error=%v output=%q", err, output.String())
	}
}

func TestThemeSwitcherMatchesProtectedComponent(t *testing.T) {
	protectedContext(t, func(ctx context.Context) {
		switcher := ThemeSwitcher{Mode: theme.Dark, Return: "/admin?q=Ada&role=Admin"}
		var output, expected strings.Builder
		if err := html.Render(ctx, &output, switcher); err != nil {
			t.Fatal(err)
		}
		if err := html.Render(ctx, &expected, components.ThemeSwitcher{
			Mode: switcher.Mode, Return: switcher.Return, CSRFField: middleware.CSRFInput{},
		}); err != nil {
			t.Fatal(err)
		}
		if output.String() != expected.String() ||
			!strings.Contains(output.String(), `name="_csrf"`) ||
			!strings.Contains(output.String(), `action="/theme"`) {
			t.Fatalf("switcher differs: %s", output.String())
		}
	})
}

func TestNavigationPropagatesWriterErrors(t *testing.T) {
	failure := errors.New("navigation writer failed")
	protectedContext(t, func(ctx context.Context) {
		for _, component := range []html.Component{
			ActionLink{Action: htmx.Action{Method: htmx.Get, URL: "/members"}, Label: "Members"},
			ThemeSwitcher{Mode: theme.Light, Return: "/members"},
		} {
			if err := html.Render(ctx, failedWriter{err: failure}, component); !errors.Is(err, failure) {
				t.Fatalf("%T: render error = %v", component, err)
			}
		}
	})
}
