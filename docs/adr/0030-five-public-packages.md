---
type: Architecture Decision Record
title: "ADR-0030: Five public packages"
status: Accepted
description: Unify MVU and persistence contracts in core and HTMX transport contracts in web.
version: 1
---

# ADR-0030: Five public packages

## Context

The seven-package layout still separates small MVU contracts from versioned
storage, and HTMX transport contracts from the HTTP runtime. During prototype
development, consumers prefer fewer import paths and accept broader packages.
Rendering safety, visual themes, and semantic components still need distinct
ownership boundaries.

## Decision

Keep exactly five public framework packages:

| Package | Ownership |
| --- | --- |
| `core` | MVU application/message/command contracts and versioned persistence |
| `html` | Direct rendering, escaping, attributes, and page/fragment contracts |
| `theme` | Embedded CSS, color modes, and preference cookies |
| `web` | Runtime, HTMX transport, forms, sessions, security, and diagnostics |
| `components` | Semantic controls, application composition, and embedded HTMX assets |

- Merge `mvu` and `store` into `core`; keep declaration names and behavior.
  Retain separate MVU and storage source files, not forwarding subpackages.
- Merge `htmx` into `web`. Use `BrowserAction` for browser attributes and keep
  `Action[Msg]` for typed runtime routes. `Action.HTMX()` returns `BrowserAction`.
- Use `HTMXRequest`, `HTMXFromRequest`, and `HTMX*` response header helpers
  to distinguish browser transport from ordinary HTTP operations.
- Keep `ErrInvalidHTMXAction` distinct from runtime `ErrInvalidAction`.
  Both error messages and existing validation behavior remain unchanged.
- Enforce dependencies:

  ```text
  core, html, theme → stdlib only
  web              → core, html, theme
  components       → html, theme, web
  ```

- `web` never imports `components`. Applications explicitly install routes,
  middleware, and assets. No templates, reflection binding, virtual DOM,
  application JavaScript, or hidden persistence.
- Application `Init`/`Update` contracts remain unchanged. Updates return
  models and commands; persistence remains explicit runtime orchestration
  outside `Update`. Co-location does not authorize storage access from updates.
- Enforce the exact public package set in `internal/arch`; examples, benchmarks,
  and private support packages are outside this count.

## Consequences

Consumers have fewer imports. HTMX-only consumers now import broader `web`.
Pure MVU consumers import `core`, which also contains storage implementation.
The former package boundary between application contracts and persistence is
gone; purity is a behavioral rule reinforced by application tests and review,
not something this package split alone can enforce.

CAS, command execution, clone policy, cookie signatures, markup, request
ordering, browser headers, styles, HTMX bytes/SRI/license, and error propagation
are unchanged. No compatibility aliases remain. See
[migration notes](../../CHANGELOG.md#unreleased).

Package layout supersedes [ADR-0029](0029-seven-public-packages.md) and the
separate MVU/store/HTMX ownership in
[ADR-0002](0002-package-boundaries.md),
[ADR-0003](0003-mvu-runtime-boundaries.md), and
[ADR-0006](0006-versioned-session-models.md), not their runtime behavior.
