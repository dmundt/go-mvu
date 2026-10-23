# ADR-0007: Provide server-controlled light and dark themes

- Status: Accepted; light default superseded by [ADR-0011](0011-system-theme-mode.md)

## Context

Applications need light and dark presentation without making components
responsible for color systems or introducing client-side application logic.

## Decision

Provide light and dark theme modes through the `theme` package. Components
define semantic structure and variants; the theme maps those variants to
presentation tokens and stylesheet behavior. The selected mode is supplied by
server-rendered application state and applied to the document.

Theme code remains independent of components and web. A theme switch is a
server-handled interaction and subsequent response; it does not require custom
application JavaScript.

## Consequences

- Components can remain visually consistent across modes without embedding
  color choices in application markup.
- Applications control how theme preference is stored and selected.
- Additional theme implementations must honor the same package boundary.

