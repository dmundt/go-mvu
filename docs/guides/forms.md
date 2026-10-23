---
type: Guide
title: Forms
description: Build typed forms with explicit decoding, validation, and errors.
version: 7
---

# Forms

Try the [contact example](../../examples/contact/) with Go 1.24 or newer:

```powershell
go run .\examples\contact
```

Open <http://localhost:8084/contact>. The example includes text fields, a
drop-down, a message box, a checkbox, validation, and a confirmation page.
It keeps entered values after errors. It works with HTMX and without
JavaScript.

## Four small steps

1. **Describe fields** with a Go struct.
2. **Read and check input** in a decoder function; no reflection or
   automatic field binding.
3. **Pass the result** into a typed app message with `web.Bind`.
4. **Show values and errors** using `components.Input`, `Select`, `Textarea`,
   and `Checkbox`. Each control accepts its label, current value, and error.

```go
type ContactDetails struct {
	Name  string
	Email string
}

type SubmitContact struct {
	Form web.Form[ContactDetails]
}
```

`web.Form` carries submitted values, field errors, and a `Valid` flag.
Keep validation in the decoder, then let `Update` decide what a valid form
does. Do not read requests or render HTML in `Update`.

`web.FormDecoder[T]` is `func(*http.Request) (T, web.FieldErrors, error)`.
Pass a function directly to `web.Bind`; field errors are ordinary
validation results, while malformed requests return an error. A nil decoder
returns `web.ErrMissingDecoder`. No interface adapter is needed.
`web.Decoder[Message]` remains the separate request-to-message contract;
`FormDecoder` returns typed input and field errors before constructing a message.
The contact example keeps request parsing and validation in
[actions.go](../../examples/contact/actions.go), separate from pure MVU updates.

## Keep route and form together

`web.Action` defines method, URL, decoder, and response once. Register it with
the runtime, then pass `action.HTMX()` to `components.ActionForm`. It selects the
matching fragment target and replaces it; ordinary form submissions redirect
to the same page. See [web actions](web-runtime.md#routes).

```go
fields := components.Fields(form.Errors)
body := components.ActionForm{
	Action: submit.HTMX(),
	Body: components.FormLayout(
		fields.Input(components.Input{
			Name: "name", Label: "Your name", Value: form.Value.Name, Required: true,
		}),
		fields.Input(components.Input{
			Name: "email", Label: "Email address", Type: "email",
			Value: form.Value.Email, Required: true,
		}),
		components.Button{Type: components.ButtonSubmit, Label: "Send"},
	),
}
```

`components.ActionForm` derives native method, URL, and HTMX behavior from one action.
Attaching HTMX to the form also covers keyboard submission. POST includes
`components.CSRFInput` automatically and reports an error if the CSRF token
is unavailable. Mount session and CSRF middleware explicitly. GET includes
no CSRF field. Only same-origin, root-relative URLs and GET/POST methods are
supported; unsupported actions return `components.ErrInvalidForm`. The lower-level
`components.Form` remains available for other explicit configurations.

For a standalone action:

```go
components.ActionButton{
	Action: increment.HTMX(),
	Label: "+",
	AriaLabel: "Increment",
}
```

It composes a secure form and submit button, preserving no-JavaScript
behavior. `Variant`, `Disabled`, and form `Attrs` remain configurable.

`components.Fields` uses each explicit control name to attach its first error.
`Text` and `Email` take name, label, current value, and required flag.
`Input`, `Select`, `Textarea`, and `Checkbox` accept existing control structs
and replace their `Error` with the matching form error. All other properties
remain unchanged. No values or validation rules are inferred. Use the
returned concrete component to customize autocomplete, hints, or attributes.

**Error replacement is unconditional:** no matching error clears any supplied
`Error`. Use named `Input` fields when positional strings or a required
boolean make the convenience calls harder to read. `Text` and `Email`
remain supported; no extra required/optional helper family is needed.

### Named-field authoring

Prefer the named control structs above for form-heavy views. They already
serve as named arguments; adding a second field configuration struct would
duplicate control properties or nest the same literal inside another wrapper.
It would not shorten the form.

An alternative is setting `Error: form.Errors.First(name)` directly on each
control. For the contact example's six controls, that adds six property lines
while removing one `fields` declaration: five extra lines with the same
one-property-per-line layout. Keep `components.Fields` when several controls share
errors. Use an explicit `Error` for isolated controls when that reads better.
No additional public field-helper family is needed.

`Select.Placeholder` prepends an escaped, selectable empty-value option:

```go
fields.Select(components.Select{
	Name: "topic", Label: "Topic", Value: form.Value.Topic,
	Placeholder: "Choose one", Options: contactTopics(), Required: true,
})
```

The placeholder is selected when `Value` is empty. A nonempty value selects
its matching option. Do not also supply an empty-value option in `Options`;
that conflict returns an error before rendering. Without `Placeholder`,
existing option rendering is unchanged. Server validation must still reject
an empty required value.

## Share field facts, not a binding schema

The contact example keeps field names, consent value, message-length limits,
and topic options in `fields.go`. Its decoder and view use those same
definitions. The empty topic placeholder is not a valid choice. Message
limits count Unicode characters, not bytes; the displayed hint uses the
same limits as server validation. Required controls still have explicit
server validation; browser checks are never sufficient.

This is application-owned data, not automatic binding or reflection.
Validation boundary tests cover accepted choices, rejected placeholders,
and exact minimum/maximum lengths. The name error message is formatted from
the same minimum-length constant used for validation.

The contact example is a learning app: requests are kept in memory for each
visitor and are not sent or stored as real support tickets. See
[production guidance](production.md) before adapting it for real submissions.
