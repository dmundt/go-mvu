---
type: Architecture Decision
title: Concise UI composition
status: Accepted
description: Small layout constructors and optional application-level integration helpers.
version: 2
---

# ADR-0021: Concise UI composition

## Context

Typed component structs preserve explicit behavior, but ordinary views repeat
child slices, document policy, form/CSRF wiring, fragment roots, and field
error lookup. Connecting these concerns inside `components` would violate
its dependencies and couple reusable controls to HTTP application setup.

## Decision

Add `Column`, `Row`, and `FormLayout` constructors to `components`. They
return existing concrete components, not a new rendering tree or DSL.
Keep struct literals available for configuration.

Add optional `ui` package above `components`, `web`, `middleware`, `static`,
`theme`, and `validation`. Existing core dependencies do not change.

- `Document` explicitly configures HTMX, CSRF headers, and validation swaps.
  `Page` uses request-local appearance; it installs no middleware or routes.
- `Form` takes one browser action for native and enhanced submission.
  It accepts same-origin root-relative GET/POST URLs only. POST requires
  CSRF context; GET omits the token. Unsupported methods return errors.
- `ActionButton` composes `Form` and a submit button.
- `ActionLink` derives native and enhanced URLs from one GET action.
- `ThemeSwitcher` accepts explicit mode and return path and supplies the CSRF
  field. It requires session/CSRF context; routes remain application-owned.
  The `theme` dependency provides the existing typed mode, not a new policy.
- `Region` supplies one fragment identity and an escaped section root.
  Unknown fragments remain errors. Region names are restricted to safe CSS
  identifiers so default action targets need no selector escaping.
- `Fields` attaches errors by explicit control name. Values, decoding, and
  validation remain application-owned.

## Consequences

Applications can adopt helpers independently without replacing existing APIs.
Security setup remains explicit at initialization. Helpers stream through
existing components and preserve escaping, sticky errors, native submission,
and theme behavior. No templates, reflection, application JavaScript, or
global registration are introduced.
