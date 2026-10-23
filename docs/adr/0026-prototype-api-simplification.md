---
type: Architecture Decision Record
title: "ADR-0026: Canonical prototype APIs"
status: Accepted
description: Remove compatibility scaffolding and resolve presentation state once.
version: 3
---

# ADR-0026: Canonical prototype APIs

Form-decoder package placement and naming are superseded by
[ADR-0028](0028-web-owned-form-validation.md).

Package placement and import names are superseded by
[ADR-0029](0029-seven-public-packages.md). The historical decision below
retains its behavioral contracts; use the seven-package layout and
[migration notes](../../CHANGELOG.md#unreleased) for current APIs.

## Context

The prototype does not need backward-compatible aliases or parallel ways to
configure identical behavior. Single-method request interfaces require adapter
types without adding a capability. Runtime theme resolution duplicates
middleware cookie resolution and silently falls back on resolver errors.
Deprecated unbounded command execution bypasses the bounded runner's admission
and shutdown contracts.

## Decision

- Use function types for `web.Decoder`, `web.KeyResolver`, `web.ExpectedVersion`,
  and `validation.Decoder`. Assign functions directly; custom implementations
  pass method values. `web.StaticMessage(message)` returns a decoder.
- Resolve theme mode in `theme.Preference.Wrap` or application middleware using
  `theme.WithMode`. Runtime and standalone `web.NewRequestContext` read that
  mode once from request context; views still receive explicit presentation
  state. Remove `RuntimeConfig.Themes` and `web.ThemeResolver`.
- Keep `NewRuntime` with explicit errors; remove `MustRuntime`.
- Use `httpx.ErrorHandler` and `httpx.RespondError` directly; remove web aliases
  and forwarding wrappers.
- Keep `SyncCommands` and bounded `AsyncCommandRunner`; remove unbounded
  `AsyncCommands`. Custom `CommandRunner` implementations remain supported.
- Keep canonical names `components.Shell`, `NavItem`, `Breadcrumb`, and
  `html.TrustedHTML`. Remove historical aliases without forwarding shims.
- Retain current package paths and dependency boundaries. Package moves without
  an ownership change would add migration cost without simplifying behavior.

## Consequences

Applications must migrate source; this is intentional during prototype
development. Cookie formats, HTML output, transition authorization, CAS retry,
outbox persistence, escaping, and bounded worker semantics are unchanged.
Theme preference middleware must wrap handlers; without it or `WithMode`,
the mode is System. There is no runtime resolver override.

This supersedes the resolver, deprecated runner, and panic-constructor portions
of [ADR-0013](0013-runtime-request-context.md). See
[migration notes](../../CHANGELOG.md#unreleased).
