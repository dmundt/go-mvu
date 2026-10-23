---
type: Architecture Decision Record
title: Default theme and deployment boundaries
description: Keep one bundled design system and delegate deployment-specific HTTP features.
version: 2
---

# ADR-0022: Default theme and deployment boundaries

Status: Accepted; middleware scope superseded by [ADR-0023](0023-ecosystem-owned-generic-middleware.md)

## Decision

- Ship only the Open Props default theme. Remove bundled Bootstrap, Bulma,
  and Pico adapters and their build dependencies. Retain the `theme.Theme`
  interface for application-owned themes and system/light/dark modes.
- Replace example framework selection with a shared mode picker.
- Remove trusted-proxy, gzip, locale-negotiation, and traceparent middleware.
  Applications or deployment infrastructure own those policies. Retain
  request bounds, security headers, and runtime diagnostics. Middleware
  inventory further reduced by ADR-0023.
- Keep fuzzing for shipped boundaries: signed session cookies.

## Consequences and migration

This is a breaking pre-stable API reduction. Applications importing removed
theme adapters must use `theme.Default` or provide their own `theme.Theme`.
Applications calling removed middleware must configure equivalent policies
outside the framework; they must not assume proxy headers are trusted or
locale/trace context is automatically populated.

ADR-0015 remains the default design-system decision. This decision supersedes
bundled-adapter support described in ADR-0010 and ADR-0011, and narrows the
deployment extension scope of ADR-0019. ADR-0023 supersedes this decision's
retained middleware inventory.
