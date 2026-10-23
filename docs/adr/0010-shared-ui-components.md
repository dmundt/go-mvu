---
type: Architecture Decision Record
title: "ADR-0010: Separate shared UI components from core components"
description: Separate shared UI components from the core component package.
version: 1
---

# ADR-0010: Separate shared UI components from core components

- Status: Accepted

## Context

Apps need common widgets: accordion, tabs, dialog, dropdown, tooltip, avatar,
progress, spinner, switch, radio group, steps, stats. React libraries ship these
with client JS. Framework forbids app JS; core `components` must stay small.

## Decision

- Put widgets in optional root package `shared`. Core `components` unchanged.
- Native HTML only: `details`/`summary`, `popover`/`popovertarget`, form
  controls, links. No JS. HTMX optional (tabs).
- Struct props, `RenderHTML`, sticky errors; same contract as `components`.
- Validate IDs, URLs (relative/http/https; explicit extras), variants, ranges;
  invalid input returns error, not markup.
- Styles in `theme/default/shared.css`, embedded into `DefaultTheme`; adapters inherit.
  Vendor conflicts fixed in `theme/<adapter>/adapter.css`.
- Class names avoid vendor collisions (`tabset`, `tip`, `progress-field`).

## Consequences

- Apps import `shared` selectively; no registration.
- All themes style widgets without markup changes.
- `Tabs` are native navigation links, not scripted ARIA tabs. `Tooltip` uses
  an auto popover with native Escape and light-dismiss behavior.
- `popover` needs current browsers.
- New widget: semantic HTML first, CSS in `theme/default/shared.css`, gallery entry,
  tests, adapter browser check.
