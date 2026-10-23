# Changelog

Notable changes to go-mvu are documented here.

## Unreleased

### Added

- `ui.ModePicker` for native GET theme previews, with explicit hidden query
  fields. Promoted from the example-only helper; gallery behavior is unchanged.

### Changed

- **Breaking:** consolidate `shared` widgets into `components`. Replace
  `github.com/dmundt/go-mvu/shared` imports and `shared.` qualifiers with
  `github.com/dmundt/go-mvu/components` and `components.`. Remove duplicate
  imports where both packages were used. No forwarding aliases remain.
- Widget validation errors now use the `components:` prefix. Markup, native
  interactions, CSS classes, and embedded widget styles remain unchanged.

### Removed

- Generic middleware duplicated by the `net/http` ecosystem: handler chaining,
  request IDs, access logging, CORS, HSTS, admission limits, and rate limits.
- The `middleware.Extension` API and bounded in-memory idempotency/replay
  middleware; use application-owned durable action ledgers when required.
- `middleware.NewRequestPolicy`, `RequestPolicyOptions`, and `NoStore`;
  applications own body limits, deadlines, content-type checks, and cache
  headers. Examples retain these protections using application-owned code.

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
- Explicit HTTP reliability policies, request diagnostics, timing, and
  middleware deployment extensions.
- Example-only stdlib JSON file persistence and atomic outbox example.
- Production OIDC example with secure sessions.
- Browser-tested UI behaviors, theme contract checks, and HTTP benchmarks.
- Architecture, security, production, component, theme, and first-app guides.
