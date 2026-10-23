---
type: Architecture Decision Record
title: "ADR-0011: System theme mode as default"
description: Use system preference as the default theme mode.
version: 2
---

# ADR-0011: System theme mode as default

- Status: Accepted

Bundled-adapter support superseded by
[ADR-0022](0022-default-theme-and-deployment-boundaries.md).
- Supersedes: light default in [ADR-0007](0007-server-controlled-themes.md)

## Context

Spec requires `system`, `light`, `dark`. Server cannot read OS preference
without client hints or JS. Light default ignored dark-mode users.

## Decision

- `theme.Mode` zero value = `System`; `Light`, `Dark` explicit.
- `system` → `data-theme="system"`; CSS resolves via
  `@media (prefers-color-scheme: dark)`. No JS.
- Default theme defines the system palette in a
  `prefers-color-scheme: dark` media block.
- Switcher = three submit buttons posting `mode` to `POST /theme`;
  `aria-pressed` marks current.
- Missing/tampered cookie → `System`.

## Consequences

- First visit matches OS theme.
- Every theme CSS must define system variant; theme tests enforce.
- `ToggleHandler` retained for legacy two-state switches.
