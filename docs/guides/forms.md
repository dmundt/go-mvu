---
type: Guide
title: Forms
description: Build typed forms with explicit decoding, validation, and errors.
version: 3
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
2. **Read and check input** in a `validation.Decoder`; no reflection or
   automatic field binding.
3. **Pass the result** into a typed app message with `validation.Bind`.
4. **Show values and errors** using `components.Input`, `Select`, `Textarea`,
   and `Checkbox`. Each control accepts its label, current value, and error.

```go
type ContactDetails struct {
	Name  string
	Email string
}

type SubmitContact struct {
	Form validation.Form[ContactDetails]
}
```

`validation.Form` carries submitted values, field errors, and a `Valid` flag.
Keep validation in the decoder, then let `Update` decide what a valid form
does. Do not read requests or render HTML in `Update`.

## Keep route and form together

`web.Action` defines method, URL, decoder, and response once. Register it with
the runtime, then pass `action.HTMX()` to `ui.Form`. It selects the
matching fragment target and replaces it; ordinary form submissions redirect
to the same page. See [web actions](web-runtime.md#routes).

```go
fields := ui.Fields(form.Errors)
body := ui.Form{
	Action: submit.HTMX(),
	Body: components.FormLayout(
		fields.Text("name", "Your name", form.Value.Name, true),
		fields.Email("email", "Email address", form.Value.Email, true),
		components.Button{Type: components.ButtonSubmit, Label: "Send"},
	),
}
```

`ui.Form` derives native method, URL, and HTMX behavior from one action.
Attaching HTMX to the form also covers keyboard submission. POST includes
`middleware.CSRFInput` automatically and reports an error if the CSRF token
is unavailable. Mount session and CSRF middleware explicitly. GET includes
no CSRF field. Only same-origin, root-relative URLs and GET/POST methods are
supported; unsupported actions return `ui.ErrInvalidForm`. The lower-level
`components.Form` remains available for other explicit configurations.

For a standalone action:

```go
ui.ActionButton{
	Action: increment.HTMX(),
	Label: "+",
	AriaLabel: "Increment",
}
```

It composes a secure form and submit button, preserving no-JavaScript
behavior. `Variant`, `Disabled`, and form `Attrs` remain configurable.

`ui.Fields` uses each explicit control name to attach its first error.
`Text` and `Email` take name, label, current value, and required flag.
`Input`, `Select`, `Textarea`, and `Checkbox` accept existing control structs
and replace their `Error` with the matching form error. All other properties
remain unchanged. No values or validation rules are inferred. Use the
returned concrete component to customize autocomplete, hints, or attributes.

## Share field facts, not a binding schema

The contact example keeps field names, consent value, message-length limits,
and topic options in `fields.go`. Its decoder and view use those same
definitions. The empty topic placeholder is not a valid choice. Message
limits count Unicode characters, not bytes; the displayed hint uses the
same limits as server validation. Required controls still have explicit
server validation; browser checks are never sufficient.

This is application-owned data, not automatic binding or reflection.
Validation boundary tests cover accepted choices, rejected placeholders,
and exact minimum/maximum lengths.

The contact example is a learning app: requests are kept in memory for each
visitor and are not sent or stored as real support tickets. See
[production guidance](production.md) before adapting it for real submissions.
