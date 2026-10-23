---
type: Architecture Overview
title: Architecture
description: Package responsibilities, dependency rules, and runtime flow.
version: 12
---

# Architecture

Go SSR framework: MVU state machine on server, HTML streamed by typed
components, HTMX as optional enhancement. No templates, no app JavaScript,
CSS inlined in `<style>`. Source of truth:
[gomvu spec](specs/gomvu-chat-spec.md); decisions: [ADRs](adr/README.md).

## Packages

| Package | Responsibility |
| --- | --- |
| `mvu` | `Application[Model, Msg]`: `Init`, `Update`, `Command`; no HTTP/HTML/storage |
| `html` | `Component`, `Page`, `Fragment`, `Context` (escaping, sticky error, streaming), `El` builder, `Attrs` allowlist |
| `htmx` | typed `Action` attrs, `Request` metadata, response headers |
| `store` | `Snapshot[M]` (`Version`), `Load`/`Create`/`CompareAndSwap`, memory store |
| `session` | signed opaque session ID → model key |
| `theme` | `Theme` (inline CSS), `Mode` system/light/dark, signed `Preference` cookie |
| `validation` | explicit field errors, no reflection |
| `components` | optional layout, controls, forms, tables, pagination, switcher, and native-HTML widgets |
| `static` | embedded HTMX + SRI hash, `/static/` handler |
| `middleware` | security headers/CSP, CSRF, stream-safe recovery, runtime diagnostics |
| `timing` | nil-safe per-request phase trace in context (decode, dispatch, load, view, render) |
| `httpx` | stdlib-only HTTP error policy, request IDs, Vary merging, response commitment |
| `web` | runtime: `Action`/route → message → Update → store → commands → view |
| `ui` | optional app composition: document policy, secure action forms, rooted regions, typed region views, field errors |
| `internal/arch` | boundary enforcement test |
| `internal/example` | private example support: server lifecycle, request policy, JSON storage, and HTTP benchmarks |
| `benchmarks` | render benchmarks vs `html/template` |

The [durable example](../examples/durable/README.md) demonstrates an
example-only JSON file store and an outbox embedded in each model record. Its
CAS operations serialize within one process; use a distributed transactional
store for multi-process deployments. This storage is not part of the framework.
See [framework positioning](guides/positioning.md).

Request limits, deadlines, accepted content types, and cache policy are
application-owned. Examples share [internal server helpers](../internal/example/server/); the copyable counter
keeps its small request wrapper locally. Framework diagnostics and
`RecoverWith` remain supported. See
[ADR-0025](adr/0025-application-owned-request-policy.md).

## Dependency rules

`examples/production` composes the example-only JSON file store with
OIDC/OAuth2 libraries. Neither is imported by core framework packages.
See [ADR-0020](adr/0020-optional-production-integrations.md) and
[production guide](guides/production.md).

Enforced by [internal/arch](../internal/arch/boundaries_test.go)
([ADR-0014](adr/0014-enforced-package-boundaries.md)).

```text
mvu, html, store, theme, static, validation, timing, httpx → stdlib only
htmx       → html
session    → store, httpx
middleware → html, session, timing, httpx
components → html, htmx, theme
web        → mvu, html, htmx, store, theme, timing, httpx
ui         → components, html, htmx, middleware, static, theme, validation, web
internal/example/jsonstore → store
internal/example/server    → httpx
examples   → framework packages and internal/example; never other examples
```

```mermaid
flowchart BT
  html; mvu; store; theme; static; validation; timing; httpx
  htmx --> html
  session --> store & httpx
  middleware --> html & session & timing & httpx
  components --> html & htmx & theme
  web --> mvu & html & htmx & store & theme & timing & httpx
  ui --> components & html & htmx & middleware & static & theme & validation & web
  app[application] --> web & components & middleware & session & static
```

Forbidden: `unsafe`; `reflect` in `html`/`components`; `html/template`,
`text/template` in library code; `mvu` → anything HTTP/HTML/storage.

Reusable widgets all live in `components`; the former `shared` package is
removed without forwarding aliases. Dependencies, native interactions, and
embedded styles remain unchanged. See [ADR-0023](adr/0023-unified-components.md).

`ui` sits above the existing layers; core packages never import it. Apps
opt in explicitly and still own middleware and asset routing. See
[ADR-0021](adr/0021-concise-ui-composition.md).

## Source organization

Prototype API policy: canonical names, function-based decoders/resolvers,
explicit startup errors, and bounded asynchronous execution. Theme mode comes
from preference/application middleware through request context; the runtime
does not resolve cookies again. See
[ADR-0026](adr/0026-prototype-api-simplification.md).

Responsibilities stay within existing packages; file boundaries do not add
dependency layers or change public APIs.

- `web`: [configuration](../web/web.go),
  [request and view contracts](../web/request.go),
  [HTTP orchestration and outcomes](../web/handler.go),
  [model transitions](../web/transition.go),
  [command execution](../web/commands.go), and
  [response rendering](../web/response.go).
- `components`: [primitives and feedback](../components/components.go),
  [documents](../components/document.go), [layout](../components/layout.go),
  [buttons and links](../components/controls.go), [forms](../components/forms.go),
  [tables](../components/table.go), and [navigation](../components/navigation.go).
  Specialized native widgets retain their existing files.
- Durable example: [lifecycle](../examples/durable/main.go),
  [service wiring](../examples/durable/service.go), and
  [persistent secrets](../examples/durable/secret.go).
- Production example: [lifecycle](../examples/production/main.go),
  [configuration](../examples/production/config.go),
  [handler wiring](../examples/production/handler.go), and
  [authentication](../examples/production/auth.go).

## Request lifecycle

```mermaid
sequenceDiagram
  participant B as Browser
  participant M as Middleware
  participant R as web.Runtime
  participant S as store
  participant A as mvu.App
  participant V as View
  B->>M: GET/POST (+HX-Request)
  M->>R: session, theme mode, CSRF checked
  R->>R: resolve key, decode typed Msg
  R->>S: Load (miss → Init → Create)
  R->>A: Update(model, msg)
  R->>S: CompareAndSwap(version) — conflict → retry ≤ N
  R->>R: CommandRunner.Run (outside store) → dispatch follow-ups
  R->>V: Page / Fragment(model, RequestContext)
  V-->>B: streamed HTML (fragment → HTMX swap, page → document)
```

1. Route method/path check → 405/404.
2. Key from session; message from `Decoder`.
3. Load, init on miss (`ErrAlreadyExists` → retry).
4. `Update` pure; CAS persist; bounded conflict retry.
5. Commands via `CommandRunner` after persist; follow-ups same path.
6. Result mode: page, fragment, 204, redirect, or `RenderAuto`.
7. View gets `RequestContext{Theme, Mode, IsHTMX, HTMX, CurrentURL}`.
8. Stream to `http.ResponseWriter`; error before first byte → `OnError`,
   after → log.

HTTP policy and runtime behavior: [middleware](guides/middleware.md),
[ADR-0019](adr/0019-http-policy-and-runtime-reliability.md).
Late panics abort the stream; ordinary render errors remain logged without
appending a replacement body. HTTP failure after CAS does not undo persistence.

## Page vs fragment

- `html.Page` = full document (`components.Document`). `html.Fragment` =
  swap target subtree. Distinct marker types; runtime cannot mix.
- `RenderAuto`: HTMX → fragment; plain form → 303 `Location`; else page.
  Same route serves HTMX and no-JS clients.

## Interaction ownership

- Server owns domain and workflow state; browser owns transient native control
  state, such as input values and expanded disclosure widgets.
- Prefer native HTML for local behavior. Use HTMX requests and stable fragment
  roots for server-backed transitions, not a client-side state mirror.
- No application JavaScript: no inline handlers or HTMX expressions either.
  HTMX itself is browser JavaScript; full-page links and forms remain the
  baseline. Push transports and additional extensions are not implied.
- Capability boundaries, fallback rules, and interaction recipes:
  [interactive applications](guides/interactions.md).

## Rendering

- Components write directly to `html.Context`: `Static`, `Text`,
  `Attribute`, numeric writers, `Trusted(TrustStatic(...))`.
- Sticky first error; no writes after failure.
- One allocation per render (context); see [benchmarks](guides/benchmarks.md).
- Composition via struct fields and slots (`Body`, `Header`, `Children`).
- Application views compose semantic components. Generic HTML and direct
  streaming stay inside named low-level components or explicit demos;
  framework component implementations can stream without a builder.
- `internal/arch/authoring_test.go` type-checks production example files and
  rejects low-level HTML APIs outside two named demo files. It permits
  `Context.Component`, `Components`, `Err`, `Fail` for composition adapters.
- Scope tests keep this authoring convention out of framework code and
  external applications; exact demo exceptions require review rationales.
  Browser-server Go tests are test infrastructure, not production views.
- Package checks recursively discover production Go files (excluding examples,
  benchmarks, testdata and dependency/hidden directories); undeclared packages
  fail, including new subpackages.
- Shared `html.Component` contract remains; no new UI language, virtual DOM,
  package relocation or compulsory rendering layer. See
  [ADR-0018](adr/0018-ui-authoring-boundaries.md).

## Themes

- `Theme.CSS()` inlined by `Document`. Mode → `data-theme`.
- `system` resolved by CSS media query; no JS.
- Default design system CSS is embedded in the `theme` package.
  See [themes](guides/themes.md).

## Constraints

- `Update`: no HTTP, no HTML, no I/O.
- Views: model → HTML only; no mutation.
- Forms decoded explicitly; validation explicit.
- Store locks never held across Update result use, commands, rendering.
- Components optional; app components live in app.
- No abstractions for hypothetical output formats.
