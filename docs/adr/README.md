---
type: Architecture Decision Catalog
title: Architecture Decision Records
description: Catalog of accepted architectural decisions.
version: 6
---

# Architecture Decision Records

This directory records the current architectural decisions for go-mvu. See
the [architecture overview](../architecture.md) for package responsibilities
and runtime flow. The [gomvu spec](../specs/gomvu-chat-spec.md) is authoritative; the
[engineering brief](../engineering-brief.md) is history. Earlier alternatives,
such as generic multi-format rendering, are not current decisions.

Use the [ADR router](index.md) to browse decisions by identifier.

## Records

| ADR | Decision | Status |
| --- | --- | --- |
| [0001](0001-html-ssr-and-htmx.md) | Keep the framework focused on Go server-rendered HTML and HTMX | Accepted |
| [0002](0002-package-boundaries.md) | Enforce package boundaries and one-way dependencies | Accepted |
| [0003](0003-mvu-runtime-boundaries.md) | Keep MVU updates pure and isolate side effects in commands | Accepted |
| [0004](0004-direct-safe-html-rendering.md) | Stream HTML directly with explicit escaping APIs | Accepted |
| [0005](0005-pages-and-fragments.md) | Distinguish full-page and fragment response contracts | Accepted |
| [0006](0006-versioned-session-models.md) | Persist versioned models behind storage and session boundaries | Accepted |
| [0007](0007-server-controlled-themes.md) | Provide server-controlled light and dark themes | Accepted |
| [0008](0008-explicit-validation.md) | Decode and validate forms explicitly without reflection | Accepted |
| [0009](0009-reusable-components.md) | Keep reusable components optional and application components local | Accepted |
| [0010](0010-shared-ui-components.md) | Separate shared UI components from core components | Package separation superseded by 0023 |
| [0011](0011-system-theme-mode.md) | System theme mode as default | Accepted |
| [0012](0012-csp-and-local-htmx.md) | Strict CSP with locally served HTMX | Accepted |
| [0013](0013-runtime-request-context.md) | Explicit request context, command runners, auto rendering | Accepted |
| [0014](0014-enforced-package-boundaries.md) | Enforce package boundaries by test | Accepted |
| [0015](0015-open-props-default-design-system.md) | Open Props default design system | Accepted |
| [0016](0016-render-timing-diagnostics.md) | Render timing and diagnostics via `timing` trace | Accepted |
| [0017](0017-element-builder-and-attrs.md) | Element builder and attribute passthrough | Accepted |
| [0018](0018-ui-authoring-boundaries.md) | Semantic composition with scoped authoring boundaries | Accepted |
| [0019](0019-http-policy-and-runtime-reliability.md) | Explicit HTTP policy, streaming lifecycle, reliable transitions | Accepted |
| [0020](0020-optional-production-integrations.md) | Example-only JSON persistence and OIDC integration | Accepted |
| [0021](0021-concise-ui-composition.md) | Small layout constructors and optional UI composition helpers | Accepted |
| [0022](0022-default-theme-and-deployment-boundaries.md) | One bundled theme and application-owned deployment policies | Accepted |
| [0023](0023-unified-components.md) | One reusable component package | Accepted |
| [0024](0024-ecosystem-owned-generic-middleware.md) | Keep generic HTTP middleware in the ecosystem | Accepted; request/cache wrapper scope superseded by 0025 |
| [0025](0025-application-owned-request-policy.md) | Keep request and cache policy in applications | Accepted |
