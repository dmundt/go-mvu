package components

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

func renderIntegration(t *testing.T, component html.Component) string {
	t.Helper()
	var output strings.Builder
	if err := html.Render(context.Background(), &output, component); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func protectedContext(t *testing.T, check func(context.Context)) {
	t.Helper()
	secret := []byte(strings.Repeat("s", 32))
	sessions, err := web.NewSession(secret, web.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	csrf, err := web.NewCSRF(secret)
	if err != nil {
		t.Fatal(err)
	}
	handler := sessions.Wrap(csrf.Wrap(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		check(r.Context())
	})))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestFormDerivesNativeAndHTMXSubmission(t *testing.T) {
	protectedContext(t, func(ctx context.Context) {
		var output strings.Builder
		form := ActionForm{
			Action: htmx.Action{Method: htmx.Post, URL: "/send", Target: "#form", Swap: htmx.OuterHTML},
			Body:   Button{Type: ButtonSubmit, Label: "Send <request>"},
			Attrs:  html.Attrs{html.A("class", "contact-form")},
		}
		if err := html.Render(ctx, &output, form); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			`method="post" action="/send" class="contact-form" hx-post="/send"`,
			`hx-target="#form" hx-swap="outerHTML"`,
			`name="_csrf" value="`,
			`type="submit"`,
			`Send &lt;request&gt;`,
		} {
			if !strings.Contains(output.String(), want) {
				t.Errorf("form missing %q: %s", want, output.String())
			}
		}
		if strings.Count(output.String(), "hx-post=") != 1 {
			t.Fatal("action must be on form only")
		}
	})

	get := renderIntegration(t, ActionForm{
		Action: htmx.Action{Method: htmx.Get, URL: "/search?q=a&sort=b"},
		Body:   Input{Name: "q", Value: "<value>"},
	})
	if !strings.Contains(get, `method="get" action="/search?q=a&amp;sort=b" hx-get=`) ||
		strings.Contains(get, "_csrf") || !strings.Contains(get, "&lt;value&gt;") {
		t.Fatalf("GET form = %s", get)
	}
}

func TestFormRejectsUnsupportedOrUnsafeActions(t *testing.T) {
	tests := []htmx.Action{
		{},
		{Method: htmx.Put, URL: "/send"},
		{Method: htmx.Patch, URL: "/send"},
		{Method: htmx.Delete, URL: "/send"},
		{Method: htmx.Post, URL: "https://outside.test/send"},
		{Method: htmx.Post, URL: "//outside.test/send"},
		{Method: htmx.Post, URL: "/\\outside.test"},
		{Method: htmx.Post, URL: "/\noutside.test"},
		{Method: htmx.Post, URL: "send"},
	}
	for _, action := range tests {
		var output strings.Builder
		if err := html.Render(context.Background(), &output, ActionForm{Action: action}); !errors.Is(err, ErrInvalidForm) {
			t.Errorf("action %+v: error = %v", action, err)
		}
		if output.Len() != 0 {
			t.Errorf("invalid action emitted HTML: %s", output.String())
		}
	}
	if err := html.Render(context.Background(), io.Discard, ActionForm{
		Action: htmx.Action{Method: htmx.Get, URL: "/send", Swap: "invalid"},
	}); !errors.Is(err, htmx.ErrInvalidAction) {
		t.Fatalf("invalid swap error = %v", err)
	}
}

func TestPostRequiresCSRFContext(t *testing.T) {
	for _, component := range []html.Component{
		ActionForm{Action: htmx.Action{Method: htmx.Post, URL: "/send"}},
		ActionButton{Action: htmx.Action{Method: htmx.Post, URL: "/send"}, Label: "Send"},
	} {
		if err := html.Render(context.Background(), io.Discard, component); !errors.Is(err, web.ErrCSRFTokenUnset) {
			t.Fatalf("render error = %v, want missing CSRF token", err)
		}
	}
}

func TestActionButtonPreservesAccessibilityAndVariant(t *testing.T) {
	protectedContext(t, func(ctx context.Context) {
		var output strings.Builder
		if err := html.Render(ctx, &output, ActionButton{
			Action:    htmx.Action{Method: htmx.Post, URL: "/minus"},
			Label:     "-",
			AriaLabel: "Decrement <count>",
			Variant:   ButtonSecondary,
			Disabled:  true,
			Attrs:     html.Attrs{html.A("aria-label", "Counter actions")},
		}); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			`aria-label="Counter actions"`, `type="submit"`, `button--secondary`,
			`aria-label="Decrement &lt;count&gt;"`, `disabled`, `name="_csrf"`,
		} {
			if !strings.Contains(output.String(), want) {
				t.Errorf("button missing %q: %s", want, output.String())
			}
		}
	})
}

func TestDocumentAppliesExplicitPolicyAndRequestAppearance(t *testing.T) {
	protectedContext(t, func(ctx context.Context) {
		document := PagePolicy{HTMX: true, CSRF: true, SwapValidationErrors: true, Lang: "de"}
		page := document.Page(web.RequestContext{
			Theme: theme.DefaultTheme{}, Mode: theme.Dark,
		}, "<Title>", Paragraph("<Body>"))
		var output strings.Builder
		if err := html.Render(ctx, &output, page); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			`lang="de" data-theme="dark"`, `&lt;Title&gt;`, `&lt;Body&gt;`,
			`src="` + HTMXPath + `"`, `integrity="` + HTMXIntegrity + `"`,
			`name="htmx-config"`, `hx-headers=`,
		} {
			if !strings.Contains(output.String(), want) {
				t.Errorf("page missing %q", want)
			}
		}
	})
	plain := renderIntegration(t, PagePolicy{}.Page(
		web.RequestContext{}, "Plain", Text("body"),
	))
	for _, absent := range []string{"<script", "hx-headers", "htmx-config"} {
		if strings.Contains(plain, absent) {
			t.Errorf("plain document unexpectedly contains %q", absent)
		}
	}
	if err := html.Render(context.Background(), io.Discard,
		(PagePolicy{HTMX: true, CSRF: true}).Page(web.RequestContext{}, "", nil),
	); !errors.Is(err, web.ErrCSRFTokenUnset) {
		t.Fatalf("document error = %v", err)
	}
}

func TestRegionPreservesRootAndUnknownFragmentError(t *testing.T) {
	region := Region("contact-form")
	body := Column(Paragraph("<hello>"))
	fragment, err := region.Select(string(region), body)
	if err != nil {
		t.Fatal(err)
	}
	want := `<section id="contact-form"><div class="stack"><p>&lt;hello&gt;</p></div></section>`
	if got := renderIntegration(t, fragment); got != want || renderIntegration(t, region.Wrap(body)) != got {
		t.Fatalf("fragment = %s, want %s", got, want)
	}
	if fragment, err := region.Select("other", body); fragment != nil || !errors.Is(err, web.ErrUnknownFragment) {
		t.Fatalf("unknown fragment = %v, %v", fragment, err)
	}
	for _, name := range []Region{"", "1form", "-form", "a b", "a.b", "a:b", `<script>`} {
		if err := html.Render(context.Background(), io.Discard, name.Wrap(body)); !errors.Is(err, ErrInvalidRegion) {
			t.Errorf("region %q error = %v", name, err)
		}
		if _, err := name.Select(string(name), body); !errors.Is(err, ErrInvalidRegion) {
			t.Errorf("Select region %q error = %v", name, err)
		}
	}
}

func TestFieldsKeepValuesAndAttachErrors(t *testing.T) {
	fields := Fields(web.FieldErrors{
		"name": {"Name <error>", "Second"}, "email": {"Email error"},
		"topic": {"Topic error"}, "message": {"Message error"}, "agreement": {"Agreement error"},
	})
	name := fields.Text("name", "Your name", "<value>", true)
	email := fields.Email("email", "Email", "invalid", true)
	if name.Error != "Name <error>" || name.Value != "<value>" || !name.Required ||
		name.Type != "text" || email.Type != "email" || email.Error != "Email error" {
		t.Fatalf("fields = %+v %+v", name, email)
	}
	for _, want := range []string{
		`for="name"`, `value="&lt;value&gt;"`, `aria-invalid="true"`,
		`aria-describedby="name-error"`, `Name &lt;error&gt;`,
	} {
		if output := renderIntegration(t, name); !strings.Contains(output, want) {
			t.Errorf("input missing %q: %s", want, output)
		}
	}
	if got := fields.Select(Select{Name: "topic", Value: "support"}); got.Error != "Topic error" || got.Value != "support" {
		t.Fatalf("select = %+v", got)
	}
	if got := fields.Textarea(Textarea{Name: "message", Value: "text"}); got.Error != "Message error" || got.Value != "text" {
		t.Fatalf("textarea = %+v", got)
	}
	if got := fields.Checkbox(Checkbox{Name: "agreement", Checked: true}); got.Error != "Agreement error" || !got.Checked {
		t.Fatalf("checkbox = %+v", got)
	}
	if got := Fields(nil).Input(Input{Name: "optional", AutoComplete: "name"}); got.Error != "" || got.AutoComplete != "name" {
		t.Fatalf("optional = %+v", got)
	}
}

type failedWriter struct{ err error }

func (writer failedWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestHelpersPropagateChildAndWriterErrors(t *testing.T) {
	want := errors.New("render failed")
	child := html.ComponentFunc(func(context.Context, *html.Context) error { return want })
	for _, component := range []html.Component{
		Region("form").Wrap(child),
		ActionForm{Action: htmx.Action{Method: htmx.Get, URL: "/search"}, Body: child},
		PagePolicy{}.Page(web.RequestContext{}, "", child),
	} {
		if err := html.Render(context.Background(), io.Discard, component); !errors.Is(err, want) {
			t.Fatalf("child error = %v", err)
		}
		if err := html.Render(context.Background(), failedWriter{want}, component); !errors.Is(err, want) {
			t.Fatalf("writer error = %v", err)
		}
	}
}
