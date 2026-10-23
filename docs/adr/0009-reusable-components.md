---
type: Architecture Decision Record
title: "ADR-0009: Keep reusable components optional"
description: Keep reusable components optional and application components local.
version: 1
---

# ADR-0009: Keep reusable components optional

- Status: Accepted

## Context

Reusable layout, form, table, and navigation components help applications,
but a privileged component system would make the core framework larger and
force application-specific concepts into a shared package.

## Decision

Keep `components` as an optional library of reusable HTML components built on
the `html`, `htmx`, and `theme` contracts. Application-specific components
remain in the application package. Components own semantic structure,
accessibility, and declared HTMX behavior; themes own presentation through
framework-neutral classes and CSS embedded in document `<style>` blocks.
Shared components must not select a CSS framework or emit inline style
attributes.

## Consequences

- Applications can adopt shared components selectively or implement their own.
- The framework core stays focused on runtime and rendering contracts.
- New shared components must be broadly reusable rather than required by the
  renderer or MVU runtime.
- Framework styling must remain independent of Bootstrap, Tailwind, or other
  application CSS frameworks.
