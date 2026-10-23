---
type: Documentation Router
title: Documentation overview
description: Entry points and recommended reading order for framework documentation.
version: 2
---

# Documentation

- New here? [Build your first app](guides/first-app.md).
- Deploy next: [Durable authenticated application](guides/production.md).

- Format: [Open Knowledge Format v0.2](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md).
  Each directory index routes to typed concepts; only root index declares
  `okf_version`.
- [Documentation router](index.md): OKF v0.2 navigation and concept entry points.
- [Spec](specs/README.md): authoritative gomvu spec, section map, deviations.
- [Architecture](architecture.md): packages, dependency rules, lifecycle
  diagrams.
- [ADRs](adr/index.md): accepted decisions through 0019.
- [Guides](guides/index.md): practical framework guides.
- [Specifications](specs/index.md): authoritative specification and map.
- [Engineering summary](engineering-summary.md): direction, status.
- Guides:
  - [Web runtime](guides/web-runtime.md): config, RequestContext, routes,
    RenderAuto, commands, middleware order.
  - [Components](guides/components.md): contract, forms, tables, pagination,
    shared widgets.
  - [Durable storage and outbox](guides/durable-store-outbox.md): persisted
    model transitions and retryable idempotent effects.
  - [Framework positioning](guides/positioning.md): target use cases and
    adjacent framework alternatives.
  - [Themes and CSS](guides/themes.md): inline CSS, tokens, system/light/dark,
    `POST /theme`, Bootstrap/Bulma/Pico adapters.
  - [Security](guides/security.md): escaping, CSP, local HTMX, CSRF, cookies.
  - [Middleware](guides/middleware.md): composition, streaming, request bounds,
    rejection policies, HTMX error handling, optional extensions.
  - [Hardening](guides/hardening.md): race/fuzz/browser gates, durable action
    example, replay access, shutdown, operational counters, response ordering.
  - [Benchmarks](guides/benchmarks.md): numbers, interpretation, rules.
- [Engineering brief](engineering-brief.md): original design conversation;
  history only.
