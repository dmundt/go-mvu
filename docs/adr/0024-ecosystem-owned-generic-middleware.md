---
type: Architecture Decision Record
title: Ecosystem-owned generic middleware
description: Keep generic HTTP middleware in the net/http ecosystem; retain go-mvu-specific policies.
version: 3
---

# ADR-0024: Ecosystem-owned generic middleware

Status: Accepted; request/cache wrapper scope superseded by
[ADR-0025](0025-application-owned-request-policy.md).

## Decision

- Keep `net/http` as the integration boundary. Runtime handlers remain
  `http.Handler`; applications compose existing middleware directly or through
  their router. Do not ship a middleware composer or framework-specific
  integrations.
- Remove generic request ID generation, access logging, CORS, HSTS, admission
  limits, and rate limits. Applications use existing middleware or deployment
  infrastructure for these concerns.
- Retain CSRF and its rendering helpers, stream-aware recovery, default
  security headers/CSP, request bounds, no-store policy, diagnostics tied to
  runtime timing.
- Retain `httpx.WithRequestID` as a low-level context bridge. External request-ID
  middleware may use it so go-mvu error, recovery, and diagnostics logs can
  include the identifier.
- Do not provide an `Extension` API or generic mutation idempotency/replay
  middleware. Applications own action deduplication and durable effects.

## Consequences

This is a pre-stable API reduction. Applications importing removed wrappers
must compose existing `net/http` middleware or configure policies at their
router or deployment edge. The production example no longer implies that a
single-process limiter is sufficient for abuse prevention. Applications that
need mutation deduplication must implement it at their durable transaction
boundary; generic buffered response replay is not supplied.

Retained middleware handles framework-specific contracts: CSRF tokens bind to
go-mvu sessions and rendered forms; recovery understands streamed response
commitment; diagnostics consume runtime phase traces; request bounds protect
framework decoders. Default security headers match local HTMX and inline
theme CSS.

This decision supersedes the deployment-middleware inventory in ADR-0022 and
the middleware-composition convenience of `middleware.Chain`. ADR-0022's theme
and other deployment-boundary decisions remain accepted.
