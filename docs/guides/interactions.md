---
type: Guide
title: Interactive applications without application JavaScript
description: Native HTML and HTMX interaction patterns, server-owned state, and capability limits.
version: 4
---

# Interactive applications without application JavaScript

Server-rendered does not mean static. CRUD, inline editing, search, tables,
workflows, and dashboards can use HTML, CSS, HTMX, and Go without application
JavaScript. Good fits include admin portals, internal tools, CRMs, inventory,
ticketing, approvals, and customer or developer portals.

go-mvu renders HTML directly through typed components, not Go templates.
Open Props supplies design tokens, not interaction behavior. HTMX is browser
JavaScript; "no application JavaScript" does not mean "JavaScript disabled."
Native controls, links, and forms supply the baseline; HTMX enhances them.

## Ownership and response flow

```text
Browser event -> HTMX request -> server decoder -> typed message
             -> pure Update -> persisted model -> rendered HTML fragment
             -> HTMX swap
```

Read-only requests can render current state without an update. Commands handle
effects outside `Update`; views render without mutation. Server owns domain
and workflow state. Browser still owns transient native state: unsent input,
focus, scroll, expanded details, and popover visibility.

Use semantic application components for views. Put low-level markup and
attribute composition inside named components, not business-view tag streams.
Use typed `htmx.Action` fields first; validated `Attrs` covers missing HTMX
attributes. See [components](components.md),
[web runtime](web-runtime.md), and [architecture](../architecture.md).

## Interaction building blocks

These are design recipes, not a claim that each is a turnkey framework
widget. Applications own endpoints, messages, validation, and authorization.

| Interaction | Composition and responsibility |
| --- | --- |
| CRUD and inline editing | GET returns read/edit region; form submits mutation; server returns updated region or field errors. Cancel links back to read view. |
| Live search | Search input sends GET with `Action.Trigger: "input changed delay:300ms, search"`; `Include` carries needed fields; replace results region. Keep enclosing GET form and submit button for non-HTMX use. |
| Autocomplete | Return server-filtered suggestions. Native `datalist` can suit simple hints; a custom combobox needs keyboard, focus, and screen-reader behavior, not merely a swapped list. |
| Cascading forms | Country selection sends GET with selected value; server returns valid state options. Revalidate country/state pair on final submit; offer submit-to-refresh fallback. |
| Filtering, sorting, pagination | GET links/forms encode query, sort, and page in URL; replace stable results region. `components.Pagination` supplies pagination; reset or clamp page after filters change. |
| Tabs and master-detail | Links select server-rendered panel or detail region. `shared.Tabs` uses server-selected link navigation, not a client-local keyboard-managed tab widget. Preserve direct URLs. |
| Accordions | `shared.Accordion` uses native `details`/`summary`; local disclosure needs no server round trip. Exclusive groups depend on browser support for `details` grouping. |
| Dialogs and slide-over panels | Native popover plus CSS suits non-modal overlays. `shared.Dialog` is a popover with dialog semantics, not a blocking modal. A server-rendered panel alone does not provide modal focus containment or inert background behavior. |
| Confirmations | `Action.Confirm` asks HTMX to show browser confirmation before request. No-HTMX or consequential workflows need a server confirmation step; confirmation is never authorization. |
| Toasts and validation feedback | Render status/error components in response; use deliberate live-region semantics. OOB fragments can update separate status region. Keep important feedback persistent; no built-in toast lifecycle is implied. |
| Multi-step wizards | Server stores validated progress; next/back forms return current step. Revalidate final submission and authorize every transition; hidden fields are not trusted workflow state. |
| Dashboards and polling | GET with `Action.Trigger: "every 5s"` refreshes region. Keep polling bounded and reads side-effect-free; configure authorization, load limits, and visible failures. |
| Infinite scrolling | GET sentinel with `Action.Trigger: "revealed"` or `"intersect once"` appends results using `BeforeEnd` or replaces sentinel using `OuterHTML`. Bound page size; advance/remove sentinel; retain next-page link. |

Native form submission supports keyboard submit. Attach mutation action to
form, not only its button, so Enter and click follow the same route. Layout,
transitions, and slide-over presentation belong in CSS files embedded into
document style blocks, never component style attributes or external
stylesheet requests.

`htmx.Action.Trigger` is a string, not a dedicated search/polling builder.
`Target`, `Include`, `Indicator`, `DisabledElt`, `PushURL`, and `ReplaceURL`
configure requests and navigation. `Swap` accepts the defined swap constants,
not arbitrary modifier strings. `hx-sync` and `hx-swap-oob` use validated
attribute passthrough; no typed fields for these are implied.

## Fragment and workflow rules

1. **Keep a working baseline.** Ordinary links navigate; forms submit without
   HTMX. Use full pages or redirects for plain requests and fragments for HTMX
   requests; see `RenderAuto` in [web runtime](web-runtime.md#result-modes).
   Provide submit/next-page controls when automatic triggers are unavailable.
2. **Define the swap contract.** Keep stable, unique region IDs. `OuterHTML`
   replacement returns its region root; `InnerHTML` returns region contents.
   Keep page and fragment rendering consistent. OOB status updates need
   separately identified targets; they are not a client state store.
3. **Keep reads safe.** GET search, polling, and pagination must not perform
   domain mutations. Mutations use appropriate methods, CSRF protection, and
   server-side authorization. Validate every request, including HTMX requests.
4. **Coordinate competing requests.** Debounce search and use `hx-sync` when
   requests can replace the same region; cancellation alone does not undo
   server mutations. Use version checks and application-owned idempotency where needed. See
   [hardening](hardening.md) and [HTTP policy](middleware.md).
5. **Make failure visible.** Preserve entered values and field errors; do not
   turn failures into empty successful fragments. HTMX does not swap most
   error responses by default. Enable `Document.HTMXSwapValidationErrors`
   explicitly for 422 field-error fragments; design other failure feedback
   deliberately. See [forms](forms.md).
6. **Preserve usability.** Labels, keyboard operation, focus after swaps,
   status announcements, loading/disabled states, and reduced-motion behavior
   need deliberate design. CSS appearance or `role="dialog"` alone does not
   supply missing behavior. Test keyboard and assistive-technology semantics
   alongside browser/computed-style checks and no-HTMX submission.
7. **Do not disguise scripts as attributes.** No inline event handlers,
   `hx-on*`, `hx-vars`, JavaScript-valued `hx-vals`/`hx-headers`, or
   JavaScript-evaluated `[...]` trigger filters. Do not weaken CSP to make
   these patterns work. See [security](security.md).

## Dialog workflows: temporary pages

Treat dialog content as a temporary page inside another page: forms,
validation, confirmations, detail views, and wizard steps remain ordinary
server-rendered components. HTMX transports HTML; it does not supply modal
semantics or automatically open an inserted native overlay.

### Content lifecycle is not modality

`shared.Dialog` renders a native popover and its trigger. It is non-modal.
`role="dialog"`, `aria-modal="true"`, CSS backdrops, or `<dialog open>` do not
make background content inert or implement modal focus containment.

A true modal needs the browser's modal-opening behavior, normally
`HTMLDialogElement.showModal()`, or a supported declarative browser mechanism.
The former needs JavaScript and is not an application-script escape hatch;
the latter needs explicit browser support, component support, and verification.
Neither is a current `shared.Dialog` guarantee.

For no-application-JavaScript overlays, prefer an existing native popover
shell with its own trigger and a stable inner content region. Trigger can
open the shell while HTMX loads that region. If the shell itself arrives in
the response, insertion alone does not show it; define an accessible native
opening step or render ordinary visible page content instead. Never promise
an on-demand blocking modal solely because its HTML is server-rendered.

### Shared target and endpoint contracts

An application can standardize one content host, such as `dialog-content`,
for create/edit forms, settings, audit details, and confirmations. Start with
an empty host or loading placeholder; only requested feature content needs
to be sent. Fetching on open avoids shipping all forms initially, but does
not guarantee freshness through a later save: recheck authorization and
model version at submission.

Use feature-owned endpoints, for example:

| Endpoint | Meaning |
| --- | --- |
| `GET /users/new` | Create form, or full page without HTMX |
| `GET /users/{id}/edit` | Authorized edit form with current values/version |
| `POST /users` | Validate and create user |
| `POST /users/{id}` | Validate and update user |
| `GET /users/{id}/delete` | Confirmation page/fragment; never deletes |
| `POST /users/{id}/delete` | Recheck permission and delete |

Same host also handles read-only detail views and server-owned wizard steps.
Each wizard submission validates current step; final mutation validates whole
workflow. No generic route registry or new framework "modal service" is
required. Full-page GETs and post/redirect/get remain fallback contracts.

For an `InnerHTML` host, make response behavior explicit:

| Outcome | Status and fragment contract |
| --- | --- |
| Open/edit/detail/next step | `200` with host contents, not another copy of host root |
| Invalid form | `422` with submitted values and field errors; enable `components.Document.HTMXSwapValidationErrors` or `ui.Document.SwapValidationErrors` |
| Successful save; remove visible panel | `200` with empty body clears host contents |
| Successful save; refresh list too | `200` with empty primary contents plus OOB list/status updates |
| Cancel/remove panel | Explicit safe GET returning empty contents, or native shell close control |

**`204` does not clear the target.** HTMX skips swapping it. Empty `200` clears
an `InnerHTML` target; empty `OuterHTML` removes target itself, losing the
shared host for future opens. Return the empty host root when replacing it
with `OuterHTML`.

Clearing content removes a panel rendered inside the host, not a persistent
outer popover/dialog shell. Shell closing is a separate native behavior;
avoid leaving an empty open overlay. Likewise, replacing an open popover
root on validation can lose its open state. Swap the inner content to retain
native shell state; test focus and error announcements after replacement.

### Save and refresh in one response

With request target `#dialog-content`, `innerHTML` swap, and existing
`#users-list` and `#save-status` targets, this illustrates response shape:

```html
<section id="users-list" hx-swap-oob="outerHTML">
  <!-- Server-rendered updated list goes here. -->
</section>
<p id="save-status" role="status" hx-swap-oob="outerHTML">User saved.</p>
```

OOB fragments update their matching targets; remaining empty primary content
clears `#dialog-content`. Emit updated list and status through escaped Go
components, not interpolated trusted strings. Keep OOB fragments top-level;
avoid duplicate IDs or unintended nested OOB processing.

This closes a panel only when clearing that host removes the panel itself.
It does not call native close on an enclosing shell. A response header event
alone also does not close anything without a listener; do not add `hx-on`
handlers to bridge this gap. All affected fragments must reflect the same
committed result; an HTTP/render failure after commit does not undo the save.

One global target requires request coordination: prevent a slow edit response
from replacing a newer detail view, and prevent late responses from reopening
canceled content. Define cancellation, duplicate-submit, back-navigation, focus
return, Escape, and loading/error behavior. Browser-test repeated open,
invalid submit, successful save/list refresh, cancel during load, keyboard
operation, and no-HTMX fallback before claiming a complete modal lifecycle.

### Runnable non-modal example

The [admin example](../../examples/admin/README.md) implements this lifecycle
without claiming true modality. Native buttons open a pre-existing popover;
HTMX loads its inner editor. Validation returns 422 and replaces only inner
content. Success replaces the workspace, removing the open shell and
installing a fresh closed one, while status updates out of band. This
deliberately avoids pretending that empty inner content closes a native shell.

Full-page Edit links provide the same form workflow without HTMX or Popover API
support. Explicit cancel clears the draft; Escape only dismisses native UI.
Browser tests exercise delayed competing responses, cancel during load,
keyboard/focus behavior, computed styles, and no-JavaScript submission.

## Live updates and SSE

Polling works with bundled HTMX core. SSE is a separate integration, not a
built-in go-mvu transport: the embedded asset does not include the HTMX SSE
extension, and the MVU runtime has no automatic subscription/push lifecycle.

Logs, notifications, monitoring, CI status, and chat-style updates can use SSE
without application-written JavaScript when an explicitly approved HTMX SSE
extension supplies browser behavior. Such an integration still ships
additional browser JavaScript. It needs a Go streaming endpoint, authenticated
and authorized subscriptions, escaped HTML events, disconnect cleanup,
reconnect/resume policy, backpressure, and proxy/timeout configuration. Review
asset serving, licensing, SRI, and CSP under [ADR-0012](../adr/0012-csp-and-local-htmx.md).
Do not present SSE as available merely by adding `sse-*` attributes.

## Fit and limits

Server-backed business interfaces can feel highly interactive without a
client-side application. Experience still depends on latency, payload size,
request frequency, concurrency, and accessible interaction design; similarity
to a named SaaS product is not a compatibility or performance guarantee.

Rich drag-and-drop boards, spreadsheet-scale editing, canvas/diagram/CAD/game
interfaces, and offline-first synchronization need substantial client-local
behavior. Do not force them into repeated fragment round trips or quietly add
application JavaScript. Revisit requirements or choose a different interaction
architecture; see [framework positioning](positioning.md).

Starting points: [counter](../../examples/counter/),
[users](../../examples/users/), [contact](../../examples/contact/), and
[component showcase](../../examples/components/). These demonstrate existing
building blocks, not every recipe above.
