---
type: Architecture Decision Record
title: Explicit HTTP policies and reliable transitions
description: HTTP lifecycle, shared rejection contracts, and commit-aware MVU policy.
version: 3
---

# ADR-0019: Explicit HTTP policies and reliable transitions

Status: Accepted

Generic middleware scope narrowed by
[ADR-0022](0022-default-theme-and-deployment-boundaries.md) and
[ADR-0024](0024-ecosystem-owned-generic-middleware.md); framework-specific
HTTP reliability decisions remain accepted.

## Context

Middleware needs defined ordering, streamed-response commitment semantics, and
consistent rejections before optional deployment features grow. MVU persistence
and command effects cannot be treated as one rollback-capable HTTP operation.

## Decision

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
- Bounded background execution and explicit deployment policies are
  application-owned. In-memory session revocation is a single-process
  convenience, not distributed security.
- Fixed operational events cross the stdlib-only `httpx` boundary, including
  session revocation saturation. Diagnostics do not depend on web/runtime types.
  The application-owned file ledger example demonstrates atomic action
  receipts/outbox descriptors without changing core storage or pure MVU APIs.

## Consequences

Applications retain control of principals, validation markup, resource
authorization, trusted proxies, and durable effect delivery. No blanket retry
of mutations, global response buffering, or telemetry vendor dependency.
The original specification stays verbatim; deviations are recorded in its map.

This refines the HTTP lifecycle in ADR-0013 and diagnostics in ADR-0016 without
replacing their MVU and package-boundary decisions.
