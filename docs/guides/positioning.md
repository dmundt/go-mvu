---
type: Guide
title: Framework positioning
description: Scope, strengths, tradeoffs, and adjacent framework choices.
version: 1
---

# Framework positioning

go-mvu is **Go-first, server-owned MVU for HTML apps**. Typed Go messages and
pure update functions feed server-side storage and direct escaped HTML
rendering. HTMX is an optional browser request/swap layer, not the state owner.

This scope fits server-owned forms, workflows, dashboards, and business
interfaces where HTML responses and Go application logic are priorities. It is
not a goal to match native UI frameworks, offline-first apps, or rich
client-side interaction.

## Adjacent frameworks

| Choice | Primary role | Choose it when |
| --- | --- | --- |
| [HTMX](https://htmx.org/docs/) | HTML attributes trigger HTTP requests and swap responses | You need hypermedia interactions; use with go-mvu or another server framework. |
| [Elm Architecture](https://guide.elm-lang.org/architecture/) | Browser-local Model / Update / View with commands and subscriptions | User interactions should update local application state without a request for each transition. |
| [templ](https://templ.guide/) | Go-native HTML component authoring compiled to Go | HTML-oriented templates, generated component code, or static rendering are central. Can be an alternative view layer, but conflicts with this repository's no-template-DSL boundary. |
| [Turbo](https://turbo.hotwired.dev/handbook/introduction) | Server-rendered navigation, frames, and streams | You want Hotwire's navigation and HTML-stream conventions. |
| [Phoenix LiveView](https://hexdocs.pm/phoenix_live_view/Phoenix.LiveView.html) | Stateful server process streams rendered diffs to connected browser | Persistent connections, server push, and diff updates justify process and transport complexity. |
| [React](https://react.dev/learn), [Vue](https://vuejs.org/guide/introduction.html), [Svelte](https://svelte.dev/docs/svelte/overview) | Client component frameworks; SSR options vary | Rich local interaction, hydration, large UI ecosystems, or client-side state are requirements. |

These tools solve overlapping but different layers. HTMX, Turbo, and templ
can be integration choices in compatible applications; Elm, React, Vue,
Svelte, and LiveView are alternative interaction/runtime models. No
cross-framework speed ranking is justified by go-mvu's current benchmarks.

## Tradeoffs and investment

- **Server-owned state:** one source of truth and ordinary HTML fallback;
  user actions that change state generally incur a request/response round trip.
- **Go-native domain logic:** no second application language or client state
  protocol; client-side offline work and high-frequency interaction remain out
  of scope.
- **Explicit rendering:** streaming escaped HTML and typed component contracts;
  unlike templ, there is no HTML template DSL or generated view code.
- **Persistence is pluggable:** `store.Memory` is process-local. The
  [JSON outbox example](../../examples/durable/README.md) demonstrates a
  simple file-backed store outside framework packages; distributed production
  stores remain application infrastructure.

Prioritize durable-store/outbox patterns, browser-tested accessibility for
shared widgets, and request benchmarks that include controlled concurrency,
latency, and persistent-store transitions. Treat measurements as workload- and
machine-specific evidence, not cross-framework claims.
