# ADR-0002: Enforce package boundaries

- Status: Accepted

## Context

The framework combines state management, HTML generation, transport, storage,
themes, and reusable components. Coupling these concerns would make the core
hard to test and replace.

## Decision

Keep the package responsibilities and dependency direction explicit:

```text
components -> html, htmx, theme
web        -> mvu, html, htmx, store, theme
session    -> store
htmx       -> html
theme      -> standard library only
mvu        -> standard library only
html       -> standard library only
store      -> standard library only
```

The `validation` and `middleware` packages provide focused supporting
capabilities. Do not let `mvu` depend on rendering, HTTP, storage, or component
packages; do not let `html` depend on application or transport packages.
Themes must not depend on components or web.

## Consequences

- Core packages remain independently testable and reusable.
- Applications compose the packages at the web boundary.
- Dependency changes that cross these borders require an explicit architectural
  justification and a revised decision.

