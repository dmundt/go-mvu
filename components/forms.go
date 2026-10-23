package components

import (
	"context"
	"fmt"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
)

type InputType string

type Input struct {
	ID           string
	Name         string
	Type         InputType
	Value        string
	Label        string
	Placeholder  string
	Required     bool
	Disabled     bool
	AutoComplete string
	Error        string
	// Attrs apply to the <input> element (not the label).
	Attrs html.Attrs
}

func (input Input) RenderHTML(_ context.Context, out *html.Context) error {
	id := input.ID
	if id == "" {
		id = input.Name
	}
	inputType := input.Type
	if inputType == "" {
		inputType = "text"
	}
	if input.Label != "" {
		out.Static("<label for=\"")
		out.Attribute(id)
		out.Static("\">")
		out.Text(input.Label)
		out.Static("</label>")
	}
	out.Static("<input class=\"input")
	out.ExtraClass(input.Attrs)
	out.Static("\" id=\"")
	out.Attribute(id)
	out.Static("\" name=\"")
	out.Attribute(input.Name)
	out.Static("\" type=\"")
	out.Attribute(string(inputType))
	out.Static("\" value=\"")
	out.Attribute(input.Value)
	out.Static("\"")
	if input.Placeholder != "" {
		out.Static(" placeholder=\"")
		out.Attribute(input.Placeholder)
		out.Static("\"")
	}
	if input.AutoComplete != "" {
		out.Static(" autocomplete=\"")
		out.Attribute(input.AutoComplete)
		out.Static("\"")
	}
	if input.Required {
		out.Static(" required")
	}
	if input.Disabled {
		out.Static(" disabled")
	}
	if input.Error != "" {
		out.Static(" aria-invalid=\"true\" aria-describedby=\"")
		out.Attribute(id + "-error")
		out.Static("\"")
	}
	out.PassAttrs(input.Attrs, "id", "name", "type", "value",
		when(input.Placeholder, "placeholder"), when(input.AutoComplete, "autocomplete"),
		whenSet(input.Required, "required"), whenSet(input.Disabled, "disabled"),
		when(input.Error, "aria-invalid"), when(input.Error, "aria-describedby"))
	out.Static(">")
	if input.Error != "" {
		out.Static("<span id=\"")
		out.Attribute(id + "-error")
		out.Static("\" class=\"field-error\" role=\"alert\">")
		out.Text(input.Error)
		out.Static("</span>")
	}
	return out.Err()
}

type Option struct {
	Value    string
	Label    string
	Disabled bool
}

type Select struct {
	ID    string
	Name  string
	Label string
	Value string
	// Placeholder adds an empty-value option before Options. Do not combine
	// it with another empty-value option.
	Placeholder string
	Options     []Option
	Required    bool
	Error       string
	// Attrs apply to the <select> element.
	Attrs html.Attrs
}

func (selectInput Select) RenderHTML(_ context.Context, out *html.Context) error {
	if selectInput.Placeholder != "" {
		for _, option := range selectInput.Options {
			if option.Value == "" {
				return fmt.Errorf("select placeholder conflicts with empty-value option")
			}
		}
	}
	id := selectInput.ID
	if id == "" {
		id = selectInput.Name
	}
	if selectInput.Error != "" && id == "" {
		return fmt.Errorf("select validation error requires ID or Name")
	}
	if selectInput.Label != "" {
		out.Static("<label for=\"")
		out.Attribute(id)
		out.Static("\">")
		out.Text(selectInput.Label)
		out.Static("</label>")
	}
	out.Static("<select class=\"input")
	out.ExtraClass(selectInput.Attrs)
	out.Static("\" id=\"")
	out.Attribute(id)
	out.Static("\" name=\"")
	out.Attribute(selectInput.Name)
	out.Static(`"`)
	if selectInput.Error != "" {
		out.Static(` aria-invalid="true" aria-describedby="`)
		out.Attribute(id + "-error")
		out.Static(`"`)
	}
	if selectInput.Required {
		out.Static(" required")
	}
	out.PassAttrs(selectInput.Attrs, "id", "name", whenSet(selectInput.Required, "required"),
		when(selectInput.Error, "aria-invalid"), when(selectInput.Error, "aria-describedby"))
	out.Static(">")
	if selectInput.Placeholder != "" {
		out.Static(`<option value=""`)
		if selectInput.Value == "" {
			out.Static(" selected")
		}
		out.Static(">")
		out.Text(selectInput.Placeholder)
		out.Static("</option>")
	}
	for _, option := range selectInput.Options {
		out.Static("<option value=\"")
		out.Attribute(option.Value)
		out.Static("\"")
		if option.Value == selectInput.Value {
			out.Static(" selected")
		}
		if option.Disabled {
			out.Static(" disabled")
		}
		out.Static(">")
		out.Text(option.Label)
		out.Static("</option>")
	}
	out.Static("</select>")
	if selectInput.Error != "" {
		out.Static("<span class=\"field-error\" id=\"")
		out.Attribute(id + "-error")
		out.Static("\" role=\"alert\">")
		out.Text(selectInput.Error)
		out.Static("</span>")
	}
	return out.Err()
}

// FormMethod is GET or POST; HTML forms support no others.
type FormMethod string

const (
	FormGet  FormMethod = "get"
	FormPost FormMethod = "post"
)

// Form renders <form> with optional CSRF field and HTMX action.
// Without Action.Method the form submits as plain HTML.
type Form struct {
	ID        string
	Method    FormMethod
	URL       string
	CSRFField html.Component
	Action    htmx.Action
	Children  []html.Component
	Attrs     html.Attrs
}

func (form Form) RenderHTML(ctx context.Context, out *html.Context) error {
	method := form.Method
	if method == "" {
		method = FormPost
	}
	if method != FormGet && method != FormPost {
		return fmt.Errorf("invalid form method %q", method)
	}
	if err := validateURL(form.URL); err != nil {
		return fmt.Errorf("invalid form URL: %w", err)
	}
	out.Static(`<form`)
	optionalAttribute(out, "id", form.ID)
	out.Static(` method="` + string(method) + `" action="`)
	out.Attribute(form.URL)
	out.Static(`"`)
	out.Class("", form.Attrs)
	if form.Action.Method != 0 {
		if err := form.Action.RenderAttributes(out); err != nil {
			return err
		}
	}
	out.PassAttrs(form.Attrs, when(form.ID, "id"), "method", "action", actionOwned(form.Action))
	out.Static(">")
	if method == FormPost {
		out.Component(ctx, form.CSRFField)
	}
	for _, child := range form.Children {
		out.Component(ctx, child)
	}
	out.Static("</form>")
	return out.Err()
}

// Label renders <label for=…>.
type Label struct {
	For   string
	Text  string
	Attrs html.Attrs
}

func (label Label) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<label")
	optionalAttribute(out, "for", label.For)
	out.Class("", label.Attrs)
	out.PassAttrs(label.Attrs, when(label.For, "for"))
	out.Static(">")
	out.Text(label.Text)
	out.Static("</label>")
	return out.Err()
}

// FieldError renders an error message linked via aria-describedby.
// Empty Message renders nothing.
type FieldError struct {
	ID      string
	Message string
	Attrs   html.Attrs
}

func (fieldError FieldError) RenderHTML(_ context.Context, out *html.Context) error {
	if fieldError.Message == "" {
		return nil
	}
	out.Static(`<span class="field-error`)
	out.ExtraClass(fieldError.Attrs)
	out.Static(`"`)
	optionalAttribute(out, "id", fieldError.ID)
	out.Static(` role="alert"`)
	out.PassAttrs(fieldError.Attrs, "role", when(fieldError.ID, "id"))
	out.Static(">")
	out.Text(fieldError.Message)
	out.Static("</span>")
	return out.Err()
}
