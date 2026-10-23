---
type: Architecture Decision Record
title: "ADR-0029: Seven public packages"
status: Superseded by ADR-0030 for package layout
description: Consolidate HTTP infrastructure and UI integration without weakening pure core boundaries.
version: 2
---

# ADR-0029: Seven public packages

The package count and separate MVU/store/HTMX ownership below are superseded
by [ADR-0030](0030-five-public-packages.md). HTTP/UI dependency direction and
behavioral contracts remain in force.

## Context

Prototype consumers should learn responsibility boundaries, not numerous small
support packages. HTTP runtime, sessions, middleware, metadata, and tracing
already form one integration surface. Semantic controls, page policy, secure
forms, and browser assets form another. Pure state, persistence, rendering,
HTMX contracts, and themes still justify independent packages.

## Decision

Keep exactly seven public framework packages:

| Package | Ownership |
| --- | --- |
| `mvu` | Pure application, message, and command contracts |
| `store` | Versioned persistence and memory implementation |
| `html` | Direct rendering, escaping, attributes, and page/fragment contracts |
| `htmx` | Browser actions, request metadata, and response headers |
| `theme` | Embedded CSS, color modes, and preference cookies |
| `web` | Runtime, forms, sessions, CSRF, recovery, diagnostics, and HTTP infrastructure |
| `components` | Semantic components, application composition helpers, and embedded HTMX assets |

- Merge `httpx`, `session`, and `middleware` into `web`; merge `ui` and
  `static` into `components`. Remove old packages without forwarding aliases.
- Preserve distinct explicit and integrated rendering contracts:
  `Document` versus `PagePolicy`, `Form` versus `ActionForm`, and
  `ThemeSwitcher` versus `SecureThemeSwitcher`. Integrated forms and switchers
  require an explicitly installed session/CSRF stack; explicit controls accept
  their own configuration.
- Keep `CSRFToken`, signing, and server protection in `web`. Move only
  `CSRFInput` and `CSRFHTMX` rendering into `components`.
- Permit `components` to import `web`, never the reverse. `web` continues to
  accept `html` interfaces and does not require the component library.
- Preserve independent `mvu`, `store`, `html`, `htmx`, and `theme` boundaries.
  Pure updates still do not inspect requests, render, or perform hidden I/O.
- Applications explicitly mount `AssetHandler`, session/CSRF middleware,
  preference handlers, and runtime routes. No automatic route installation.
- Enforce exact public package set and allowed dependencies in `internal/arch`.
  Examples, benchmarks, and internal support packages are outside this count.

## Consequences

Consumers use fewer imports. `web` and `components` are broader packages;
focused files preserve internal responsibility boundaries. Importing
`components` also includes its HTTP integration and embedded HTMX asset.

This is a breaking prototype migration, not a compatibility layer. See
[migration notes](../../CHANGELOG.md#unreleased) for renamed constructors,
types, controls, and asset routing.

Session cookie formats, CSRF signatures, persistence, direct rendering,
markup, styles, HTMX bytes/SRI/license, request ordering, and error propagation
remain unchanged.

Package placement supersedes the separate supporting layers in
[ADR-0002](0002-package-boundaries.md),
[ADR-0006](0006-versioned-session-models.md),
[ADR-0012](0012-csp-and-local-htmx.md),
[ADR-0019](0019-http-policy-and-runtime-reliability.md),
[ADR-0021](0021-concise-ui-composition.md),
[ADR-0023](0023-unified-components.md),
[ADR-0024](0024-ecosystem-owned-generic-middleware.md),
[ADR-0026](0026-prototype-api-simplification.md), and
[ADR-0027](0027-consolidated-http-observability.md), not their behavioral
contracts. [ADR-0028](0028-web-owned-form-validation.md) remains unchanged.
