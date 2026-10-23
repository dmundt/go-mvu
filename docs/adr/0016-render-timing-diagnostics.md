# ADR-0016: Render timing and diagnostics

- Status: Accepted
- Extends: [ADR-0013](0013-runtime-request-context.md),
  [ADR-0014](0014-enforced-package-boundaries.md)

## Context

Need per-request render time and phase breakdown (decode, update, store,
view, render) for performance work. Middleware sees only the whole request;
phases live inside the web runtime. Runtime must not import `middleware`
(outer layer). Rendering streams, so render time is unknown when headers go
out.

## Decision

- New leaf package `timing` (stdlib only): nil-safe `Trace` in context.
- `web` records phases `decode`, `dispatch`, `load`, `view`, `render`.
- `middleware.Diagnostics` attaches the trace; adds `ttfb`, `total`; logs via
  slog (Warn when slow); keeps in-memory aggregates; JSON `Handler`.
- Optional `Server-Timing`: header for pre-body phases, trailer for `render`
  and `total`. Default off.
- Allowlist: `timing` → none; `web`, `middleware` → `+timing`.

## Consequences

- No trace → one context lookup per phase, no allocation; streaming intact.
- Render time in browser devtools only for pre-body phases unless the client
  reads trailers; logs and `Handler` always have it.
- Aggregate keys = phase names; apps adding dynamic phase names grow the map.
  Use fixed names.
- `Handler` and `Server-Timing` leak timing data; internal/dev use only.
