# ADR-0013: Explicit runtime request context, command runners, auto rendering

- Status: Accepted
- Extends: [ADR-0003](0003-mvu-runtime-boundaries.md),
  [ADR-0005](0005-pages-and-fragments.md)

## Context

Views read theme from context implicitly. Commands ran inside retry loop.
Plain form posts and HTMX needed separate routes.

## Decision

- Views receive `web.RequestContext{Theme, Mode, IsHTMX, HTMX, CurrentURL}`.
- `ThemeResolver` interface (`Resolve`, `Persist`); `theme.Preference`
  implements it.
- `CommandRunner[Msg]`: `SyncCommands` default, `AsyncCommands` detached.
  Commands run after successful CAS, outside store calls; follow-up
  messages take same transition path.
- `RenderAuto`: HTMX → fragment; `Location` → 303; else page.
- `NewRuntime` validates config, returns error; `MustRuntime` for startup.
- Errors after first byte logged, not passed to `OnError`.

## Consequences

- Views testable without HTTP context setup.
- One route serves HTMX and no-JS clients.
- Async commands lose request cancellation by design; errors via callback.
