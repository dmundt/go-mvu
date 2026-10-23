---
type: Architecture Decision Record
title: Default theme and deployment boundaries
description: Keep one bundled design system and delegate deployment-specific HTTP features.
version: 1
---

# ADR-0022: Default theme and deployment boundaries

Status: Accepted

## Decision

- Ship only the Open Props default theme. Remove bundled Bootstrap, Bulma,
  and Pico adapters and their build dependencies. Retain the `theme.Theme`
  interface for application-owned themes and system/light/dark modes.
- Replace example framework selection with a shared mode picker.
- Remove trusted-proxy, gzip, locale-negotiation, and traceparent middleware.
  Applications or deployment infrastructure own those policies. Retain
  request bounds, security headers, CORS, direct-TLS HSTS, rate limits,
  idempotency, and runtime diagnostics.
- HSTS checks direct TLS only. TLS-terminating proxies must set HSTS at the
  trusted edge; raw forwarding headers never establish secure transport.
- Keep only fuzz jobs for shipped boundaries: action IDs and session cookies.

## Consequences and migration

This is a breaking pre-stable API reduction. Applications importing removed
theme adapters must use `theme.Default` or provide their own `theme.Theme`.
Applications calling removed middleware must configure equivalent policies
outside the framework; they must not assume proxy headers are trusted or
locale/trace context is automatically populated.

ADR-0015 remains the default design-system decision. This decision supersedes
bundled-adapter support described in ADR-0010 and ADR-0011, and narrows the
deployment extension scope of ADR-0019; their remaining decisions stand.
