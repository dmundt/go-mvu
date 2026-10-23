---
type: Architecture Decision Record
title: Explicit HTTP policies and reliable transitions
description: HTTP lifecycle, shared rejection contracts, and commit-aware MVU policy.
version: 1
---

# ADR-0018: Explicit HTTP policies and reliable transitions

Status: Accepted

## Context

Middleware needs defined ordering, streamed-response commitment semantics, and
consistent rejections before optional deployment features grow. MVU persistence
and command effects cannot be treated as one rollback-capable HTTP operation.

## Decision

- Keep ordinary `func(http.Handler) http.Handler` composition. `Chain` lists
  execution order without sorting or registration.
- Add stdlib-only `httpx` contracts shared by middleware and the web runtime.
  Keep HTTP, persistence, and rendering out of `mvu`.
- Recovery replaces only uncommitted responses; late panics abort. Completion
  observations execute during unwinding.
- Bound request reads and propagate cooperative cancellation. Session HTML
  defaults to no-store. Static assets do not create sessions.
- Preserve HTTP error statuses. Explicit HTMX meta configuration enables 422
  validation swaps without application JavaScript.
- Authorization and expected-version checks belong at the transition boundary;
  command follow-ups use the same authorization path. Record commit-aware
  outcomes; failures after persistence do not imply rollback.
- Bounded background execution and optional deployment policies are explicit.
  In-memory idempotency and revocation are single-process conveniences, not
  durable exactly-once or distributed security guarantees.

## Consequences

Applications retain control of principals, validation markup, resource
authorization, trusted proxies, and durable effect delivery. No blanket retry
of mutations, global response buffering, or telemetry vendor dependency.
The original specification stays verbatim; extensions are recorded in its map.

This refines the HTTP lifecycle in ADR-0013 and diagnostics in ADR-0016 without
replacing their MVU and package-boundary decisions.
