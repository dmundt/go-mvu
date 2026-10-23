# Changelog

Notable changes to go-mvu are documented here.

## Unreleased

### Removed

- Generic middleware duplicated by the `net/http` ecosystem: handler chaining,
  request IDs, access logging, CORS, HSTS, admission limits, and rate limits.
- The `middleware.Extension` API and bounded in-memory idempotency/replay
  middleware; use application-owned durable action ledgers when required.
- Legacy shared component aliases; use types from `components`.

Applications can compose existing middleware directly with go-mvu handlers.
Framework-specific CSRF, stream-aware recovery, security headers, request
bounds, and diagnostics remain available. Use application-owned durable action
ledgers or outboxes when mutation replay guarantees are required.

## [0.1.0] - 2026-10-23

Initial release.

### Added

- Go-first, server-owned MVU runtime for directly rendered, escaped HTML with
  HTMX progressive enhancement.
- Typed components, pages, fragments, validation, middleware policies, themes,
  and reusable UI components.
- Explicit HTTP reliability policies, request diagnostics, and timing.
- Optional CASK-backed durable model storage and atomic outbox example.
- Production OIDC example with secure sessions.
- Browser-tested UI behaviors, theme contract checks, and HTTP benchmarks.
- Architecture, security, production, component, theme, and first-app guides.
