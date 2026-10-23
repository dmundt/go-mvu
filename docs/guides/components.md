---
type: Guide
title: Components and pagination
description: Contracts and usage for reusable components and pagination.
version: 2
---

# Components and pagination

`Document.HTMXSwapValidationErrors` explicitly enables 422 validation fragment
swaps through an escaped HTMX configuration meta element. Other error statuses
keep default no-swap behavior. See [HTTP policy](middleware.md#cache-and-htmx-policy).

Reusable components are optional helpers, not a required rendering layer.
Applications can combine `components` package elements with local components;
local, business-specific components should stay with the application. See
[ADR-0009](../adr/0009-reusable-components.md).

## Authoring levels

| Level | API | Preferred location |
| --- | --- | --- |
| Semantic composition | `components`, `shared`, application components | Application views |
| Generic HTML | `html.El`, `Attrs`, `Text` | Named low-level components; explicit escape-hatch demos |
| Direct streaming | `Context.Static`, escaped writers, numeric writers | Framework primitives; measured hot paths; explicit low-level components |

All levels share `html.Component`. Interoperability stays supported; ordinary
views should not alternate between tag emission and semantic composition.
Keep low-level details behind a named component, not an inline
`ComponentFunc` in a business view. Use typed properties first; `Attrs` extends
missing attributes, not layout APIs or component-owned names.

Repository examples enforce this policy with a type-aware architecture test.
Only [theme markup](../../examples/themes/lowlevel.go) and
[element interoperability](../../examples/components/lowlevel.go) are exempt.
Framework component implementations remain free to render directly; no
mandatory builder layer. Applications may adopt this policy without adopting
the repository's test. See [ADR-0018](../adr/0018-ui-authoring-boundaries.md).
Scope tests ensure framework packages and external applications remain outside
this policy. New exceptions need an exact file path and review rationale;
renaming a file `lowlevel.go` alone never exempts it. Go test files, including
browser-server harnesses, are not application authoring surfaces.

## Component contract

Implement `html.Component` to render any HTML subtree. Embed `html.PageMarker`
for a complete document or `html.FragmentMarker` for a subtree returned from
an HTMX endpoint:

```go
type Greeting struct {
	html.FragmentMarker
	Name string
}

func (g Greeting) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, components.Paragraph("Hello, "+g.Name))
	return out.Err()
}
```

Semantic components own escaping. In low-level implementations, use `Static`
only for developer-controlled markup; `Text` and `Attribute` escape values.
Nested components should be rendered
through `out.Component(ctx, child)` so the render context retains its first
error.

Components can define slots with ordinary fields:

```go
components.Card{
	Header: components.Text("Users"),
	Body:   userTable,
	Footer: components.Pagination{ /* page configuration */ },
}
```

Nil slots are optional. A component owns semantic HTML and accessibility;
theme classes should describe roles and variants, not a CSS framework.

## Shorter composition

`Column(...)` and `Row(...)` return `Stack` and `Inline`. They remove child
slice boilerplate without adding a new rendering model:

```go
components.Column(
	components.Heading{Level: 2, Text: "Contact"},
	components.Row(
		components.Paragraph("Questions?"),
		components.Link{URL: "/contact", Label: "Write to us"},
	),
)
```

Use `Stack{Gap: ..., Attrs: ...}` and `Inline{Attrs: ...}` for custom
configuration. `FormLayout(...)` returns a stack whose inputs, selects, and
textareas fill the available width; labels and field errors remain separate,
accessible components. Its CSS works with all shipped themes.

### Shared page policy

The optional `ui` package connects existing components and HTTP helpers.
Configure document policy once when constructing your view:

```go
type View struct{ Document ui.Document }

view := View{Document: ui.Document{
	HTMX: true,
	CSRF: true,
	SwapValidationErrors: true,
}}
```

Inside `Page`, return `view.Document.Page(request, "Contact", body)`.
It uses the request's theme and mode and the embedded HTMX source and SRI.
`CSRF` enables the header wrapper when HTMX is enabled. The application
must still mount `static.Handler`, session middleware, and CSRF middleware;
the helper never installs routes or protection. A zero `ui.Document` emits
plain pages without HTMX or the header wrapper.

### Rooted fragments

Use a region for a stable fragment name and its section root:

```go
const contactRegion ui.Region = "contact-form"

// In the shared web.Action:
result := web.Result{Mode: web.RenderAuto, Fragment: string(contactRegion)}

// On the full page:
body := contactRegion.Wrap(ContactForm{Form: model.Form})

// In View.Fragment:
return contactRegion.Select(name, ContactForm{Form: model.Form})
```

`Wrap` and `Select` render the same section ID for `outerHTML` replacement.
`Select` reports `web.ErrUnknownFragment` for other names. Names start with
an ASCII letter or underscore, followed by letters, digits, underscores, or
hyphens; invalid names report `ui.ErrInvalidRegion`. For multiple regions,
dispatch to the matching region explicitly. Helpers preserve rendering
errors and escape child content.

See [forms](forms.md) for secure action forms and explicit field helpers.
Existing `Document`, `Form`, control structs, and custom fragment types
remain supported. See [ADR-0021](../adr/0021-concise-ui-composition.md).

## Low-level element builder

`html.El` builds any element without hand-written tag strings. Attributes
are an ordered `html.Attrs` list; text goes through `html.Text`. Elements and
components nest both ways. This is an explicit low-level example, not the
default application authoring style:

```go
html.El("section", html.Attrs{html.A("class", "panel"), html.A("id", "release")},
	html.El("h2", nil, html.Text("Release")),
	html.El("time", html.Attrs{html.A("datetime", "2025-01-15")}, html.Text("Jan 15")),
	html.El("input", html.Attrs{html.A("name", "q"), html.Flag("required")}),
	components.Button{Label: "Save", Type: components.ButtonSubmit},
)
```

- Values escaped; boolean attributes via `html.Flag`.
- Void elements (`br`, `img`, `input`, …) get no end tag; children → error.
- `script`, `style`, `link`, `iframe` and other active tags →
  `html.ErrInvalidElement`.
- Unsafe attributes → `html.ErrUnsafeAttribute` (see
  [security](security.md#attribute-allowlist)).
- `htmx.Action{...}.Attrs()` returns the `hx-*` list for `El`.

## Attribute passthrough

Root-bearing struct components generally expose `Attrs html.Attrs`:

```go
components.Card{
	Body: stats,
	Attrs: html.Attrs{
		html.A("class", "card--wide"),
		html.A("data-row", "7"),
		html.A("hx-get", "/stats"),
		html.A("hx-trigger", "every 30s"),
	},
}
// <article class="card card--default card--wide" data-row="7" hx-get="/stats" …>
```

- Root element gets Attrs. Exceptions: form controls (`Input`, `Select`,
  `Checkbox`, `Textarea`, `Switch`) → the control; `Dialog` → popover panel;
  `Table` → `<table>`; `Shell` → `div.shell`.
- All non-empty `class` values merge into component classes, in input order.
  Repeated class tokens are preserved; blank and flag-only class entries add
  no value.
- Component-owned names win and are skipped: e.g. `type` on `Button`,
  `href` on `Link`, `role` on `Alert`, `id` when `ID` is set, all `hx-*`
  when `Action` is set (including `data-hx-*` aliases).
- Other repeated names are case-insensitive first-value-wins: only the first
  is emitted, in its original position and casing. `hx-get` and `data-hx-get`
  count as one name; unrelated `data-*` names remain independent. The same
  duplicate policy applies to `html.El` / `Context.Attrs`.
- Owned names and later duplicate values are discarded before emission;
  validation applies to attributes actually emitted. A false/unset optional
  property owns nothing unless the component still writes a default.
- Same allowlist as `html.El`.

Exceptions retained for compatibility:

| API | Reason / alternative |
| --- | --- |
| `Document` | Owns document metadata/theme attributes; use typed fields and `Head`/`Body` slots. No arbitrary root passthrough. |
| `Paragraph` (string), `HeaderRow` (slice) | Convenience APIs, no `Attrs` field. For extra attributes, encapsulate `html.El("p", ...)` in a named component, or compose `TableRow` + `TableCell`. |
| `Text`, `Empty`, `Sequence`, `html.Optional` | No owned root element. Put attributes on the enclosing component. |
| `Option`, `NavItem`, `Breadcrumb`, shared item records | Configuration data, not independently rendered components. Parent component owns markup. |

Inside a low-level custom component, use the same helpers:

```go
out.Static(`<div class="panel`)
out.ExtraClass(panel.Attrs)
out.Static(`"`)
out.PassAttrs(panel.Attrs, "role")
out.Static(` role="region">`)
```

## Browser regression checks

Run `npm ci --ignore-scripts`, `npx playwright install chromium --only-shell`,
then `npm run test:browser`. Go consumers do not need Node or browser tooling.
The suite uses real example handlers through opt-in Go test servers, not mocks.
It starts five loopback servers on ports 18080–18084 and stops them after the
run; occupied ports fail startup rather than reusing an unrelated process.

Projects: `desktop-light`, `desktop-dark`, `mobile-light`, `mobile-dark`
(1280px / 390px). Every project covers all four themes with actual computed
styles, native popover open/close, and theme-form navigation preserving tabs.
Counter and pagination assert HTMX preserves the document. Pagination also
checks Back/Forward restoration; no-JavaScript counter forms and pagination
verify full-page fallback and reload persistence. User cards and plain tables
also get computed-style checks.

Each theme checks keyboard tab order, visible focus, Enter/Space dialog
activation, Escape dismissal, focus restoration, dialog accessible names and
tooltip descriptions. Tooltips activate with Enter/Space or click and dismiss
with Escape or an outside click. System mode must match explicit palettes under
both OS preferences and respond to live preference changes. Reduced motion
keeps tooltips motion-free and must limit spinners to one iteration lasting
at most 1ms. No arbitrary sleeps or selector-only styling assertions.

Focused run: `npm run test:browser -- --project=mobile-dark`.
If browser installation is unavailable locally, set `PLAYWRIGHT_CHANNEL` to
`msedge` or `chrome` to use an already installed Chromium-based browser;
CI uses the pinned Playwright browser with no channel override.
The UI matrix has 64 checks, alongside request-ordering tests.
CI runs each project as a separate matrix job with
pinned Chromium Headless Shell; browser jobs stop after 20 minutes and Go jobs
after 15 minutes, including server startup and teardown. Failures retain traces and
screenshots in ignored `test-results/` and an HTML report in
`playwright-report/`. Browser automation JavaScript is test-only, never served
to applications.

## Shared UI widgets

Optional package `shared` holds React-style widgets with native HTML, no JS.
See [ADR-0010](../adr/0010-shared-ui-components.md). Gallery:
`go run .\examples\components` → `http://localhost:8083`.

| Group | Components | Native basis |
| --- | --- | --- |
| Overlay | `Accordion`, `Tabs`, `Dialog`, `Dropdown`, `Tooltip` | `details`, native links, auto-dismiss popovers |
| Display | `Avatar`, `Progress`, `Spinner`, `Skeleton`, `Separator`, `Kbd`, `Stat`, `Steps` (+ aliases `Inline`, `EmptyState`) | `progress`, `role=status`, `dl`, `ol` |
| Forms | `Switch`, `RadioGroup` (+ aliases `Checkbox`, `Textarea`) | `input[role=switch]`, `fieldset`, `textarea` |

```go
shared.Dialog{
	ID: "invite", Title: "Invite teammate", TriggerLabel: "Open dialog",
	Body: form, Footer: actions,
}
```

- Invalid IDs, URLs, variants, ranges → render error.
- Styles: `theme/default/shared.css`, included in every theme.
- `Tabs` are labelled navigation links with `aria-current`, not scripted ARIA
  tabs; browser link keyboard behavior and full-page fallback remain native.
- `Tooltip` uses an auto popover tied to its button with `aria-describedby`;
  native Escape and light-dismiss behavior work without application JavaScript.

## Reusing application components

A custom component is reusable anywhere that imports its Go package. It needs
no registration, global state, or framework extension point. Keep components
that are useful across applications in a normal Go package; keep business
specific components in their application package.

For example, a reusable `usercard` package can expose:

```text
internal/ui/usercard/
    card.go
```

```go
package usercard

import (
	"context"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
)

type Card struct {
	Name  string
	Email string
}

func (card Card) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, components.Card{
		Attrs: html.Attrs{html.A("class", "user-card")},
		Body: components.Stack{Children: []html.Component{
			components.Heading{Level: 2, Text: card.Name},
			components.Paragraph(card.Email),
		}},
	})
	return out.Err()
}
```

Compose it like any other component:

```go
components.Stack{
	Children: []html.Component{
		usercard.Card{Name: user.Name, Email: user.Email},
		components.Badge{Label: user.Role},
	},
}
```

Put reusable composition in the component package. `Card` owns the base
presentation. Additional business-specific styling belongs in the
application's `theme.Theme.Stylesheet()` implementation, not inline styles:

```css
.user-card {
  max-width: 40rem;
}
```

The application passes its theme to `components.Document`; the framework
embeds that theme's CSS in the document. This keeps the component package
independent of CSS frameworks and lets different applications theme the same
component differently. `Static` is only for fixed markup; render all dynamic
values with `Text` or `Attribute`.

The runnable [`UserCard`](../../examples/users/card.go) is
used by the users example at `/users/cards`; its tests verify that untrusted
card fields are escaped.

## Shared component inventory

The `components` package currently includes:

- Composition and layout: `Empty`, `Text`, `Sequence`, `Document`, `Shell`
  (alias `AppShell`: `Header`, `Sidebar`, `Main`, `Footer` landmarks),
  `Section` (`ID`, `Children`, `Attrs`), `Stack`, `Grid`, `Inline`.
  Standalone landmarks: `Header`, `Sidebar{AriaLabel}`,
  `Main`, `Footer`. `Document.HTMXSource` (`static.HTMXPath`) + `HTMXIntegrity` → local HTMX
  script with SRI.
- Primitives: `Heading` (level 1–6), `Paragraph`, `Output` (`Live` adds
  `aria-live`).
- Content: `Card`, `Alert`, `Badge`, `EmptyState{Title, Description, Action}`.
- Controls: `Button` (`ID`, `AriaLabel`, HTMX `Action`), `Link` (`Variant`
  renders a button-styled link), `Input`, `Select`, and `ThemeSwitcher`.
- Forms: `Form` (`FormGet`/`FormPost`; `CSRFField` rendered for POST only;
  optional HTMX `Action`), `Label`, `FieldError` (empty → nothing),
  `Checkbox`, `Textarea` (hint + error via `aria-describedby`).
- Data and navigation: `Table`, `TableRow`, `TableCell` (`Header`,
  `Scope`), `HeaderRow`, `TableHeader`, `TableBody`, `Pagination`,
  `Breadcrumbs` (`BreadcrumbItem`), `Navigation` (`NavigationItem`).

Prefer primitives over hand-written `Static` tags. Example:

```go
components.Button{
	Label:     "+",
	AriaLabel: "Increment",
	Action:    htmx.Action{Method: htmx.Post, URL: "/counter/increment"},
}
```

When an action also has a Go route, define it as `web.Action` and pass
`action.HTMX()` to the control. This keeps its method, path, and fragment in
one place; see [web actions](web-runtime.md#routes).

Components stream output directly. `Table` supplies `table`/`thead`/
`tbody`/`tfoot`; rows built from `TableRow` + `TableCell`:

```go
components.Table{
	Header: components.HeaderRow{"Name", "Email"},
	Body:   components.Sequence(rows), // []html.Component of TableRow
}
```

`Table` output wrapped in `<div class="table-scroll">` (horizontal scroll on
narrow screens). `Sections []html.Component` replaces `Header`/`Body`/`Footer`:

```go
components.Table{Sections: []html.Component{
	components.TableHeader{Rows: headerRows},
	components.TableBody{Rows: rows},
}}
```

No-JS fallback: wrap HTMX buttons in `Form{Method: FormPost, URL: ...}`;
route uses `web.RenderAuto` + `Location` (303 for plain posts).

## Pagination

`Pagination` emits ordinary links for previous, numbered, and next pages. When
`Target` is set, it adds an HTMX GET action to each link; without HTMX those
links still navigate normally. The application supplies a URL function or a
`BaseURL` and `QueryName`.

- ≤ 1 page → renders nothing.
- Layout: prev, 1, …, window around current, …, last, next.
- `Current()` clamps page into `1..Pages()`; prev/next carry `rel`.
- Active page: `aria-current="page"`.

```go
components.Pagination{
	Page:          model.Page,
	PageSize:      model.PageSize,
	TotalItems:    model.TotalItems,
	Window:        2,
	BaseURL:       "/users",
	QueryName:     "page",
	Target:        "#users-results",
	Swap:          htmx.OuterHTML,
	PushURL:       true,
	PreviousLabel: "Previous",
	NextLabel:     "Next",
	AriaLabel:     "Users pagination",
}
```

Alternatively, supply `URL: func(page int) string { ... }` when routing needs
custom query parameters. The component validates generated URLs before writing
them. Keep requested page parsing, bounds checking, and result loading on the
server.

The complete runnable pagination flow is in
[`examples/users`](../../examples/users/view.go). It returns the full page for
normal requests and only the results fragment for HTMX requests.

## Layout spacing

`Stack.Gap` and `Grid.Gap` map a small set of spacing values to framework
classes; arbitrary CSS values are rejected. Supported values: `0`, `0.25rem`,
`0.5rem`, `1rem`, `1.5rem`, and `2rem`, plus their supported pixel equivalents.
`Grid.Columns` supports 1 through 12. These constraints keep component markup
free of inline style attributes.
