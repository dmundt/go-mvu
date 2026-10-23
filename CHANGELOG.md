# Changelog

Notable changes to go-mvu are documented here.

## Unreleased

### Changed

- **Breaking:** consolidate `shared` widgets into `components`. Replace
  `github.com/dmundt/go-mvu/shared` imports and `shared.` qualifiers with
  `github.com/dmundt/go-mvu/components` and `components.`. Remove duplicate
  imports where both packages were used. No forwarding aliases remain.
- Widget validation errors now use the `components:` prefix. Markup, native
  interactions, CSS classes, and embedded widget styles remain unchanged.

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
- Production OIDC example with secure sessions and rate limits.
- Browser-tested UI behaviors, theme contract checks, and HTTP benchmarks.
- Architecture, security, production, component, theme, and first-app guides.
