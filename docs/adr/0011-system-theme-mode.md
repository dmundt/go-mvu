# ADR-0011: System theme mode as default

- Status: Accepted
- Supersedes: light default in [ADR-0007](0007-server-controlled-themes.md)

## Context

Spec requires `system`, `light`, `dark`. Server cannot read OS preference
without client hints or JS. Light default ignored dark-mode users.

## Decision

- `theme.Mode` zero value = `System`; `Light`, `Dark` explicit.
- `system` → `data-theme="system"`; CSS resolves via
  `@media (prefers-color-scheme: dark)`. No JS.
- Build script clones adapter dark rules into system media block
  (doubled attribute selector for specificity; `:not(dark)` rules skipped).
- Switcher = three submit buttons posting `mode` to `POST /theme`;
  `aria-pressed` marks current.
- Missing/tampered cookie → `System`.

## Consequences

- First visit matches OS theme.
- Every theme CSS must define system variant; theme tests enforce.
- `ToggleHandler` retained for legacy two-state switches.
