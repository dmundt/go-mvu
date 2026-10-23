package components

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

func TestActionLinkSharesEscapedURLAndPresentation(t *testing.T) {
	output := renderIntegration(t, ActionLink{
		Action: web.BrowserAction{
			Method: web.Get, URL: "/edit?id=1&q=%3Cname%3E&role=Admin",
			Target: "#members", Swap: web.OuterHTML, PushURL: "true",
		},
		Label: "Edit <member>", Variant: ButtonSecondary,
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
		action web.BrowserAction
		err    error
	}{
		{web.BrowserAction{}, ErrInvalidLink},
		{web.BrowserAction{Method: web.Post, URL: "/save"}, ErrInvalidLink},
		{web.BrowserAction{Method: web.Get, URL: "javascript:alert(1)"}, web.ErrInvalidHTMXAction},
		{web.BrowserAction{Method: web.Get, URL: "/edit", Swap: "invalid"}, web.ErrInvalidHTMXAction},
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
	err := html.Render(context.Background(), &output, SecureThemeSwitcher{Mode: theme.Dark, Return: "/"})
	if !errors.Is(err, web.ErrCSRFTokenUnset) || output.Len() != 0 {
		t.Fatalf("error=%v output=%q", err, output.String())
	}
}

func TestThemeSwitcherMatchesProtectedComponent(t *testing.T) {
	protectedContext(t, func(ctx context.Context) {
		switcher := SecureThemeSwitcher{Mode: theme.Dark, Return: "/admin?q=Ada&role=Admin"}
		var output, expected strings.Builder
		if err := html.Render(ctx, &output, switcher); err != nil {
			t.Fatal(err)
		}
		if err := html.Render(ctx, &expected, ThemeSwitcher{
			Mode: switcher.Mode, Return: switcher.Return, CSRFField: CSRFInput{},
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
			ActionLink{Action: web.BrowserAction{Method: web.Get, URL: "/members"}, Label: "Members"},
			SecureThemeSwitcher{Mode: theme.Light, Return: "/members"},
		} {
			if err := html.Render(ctx, failedWriter{err: failure}, component); !errors.Is(err, failure) {
				t.Fatalf("%T: render error = %v", component, err)
			}
		}
	})
}
