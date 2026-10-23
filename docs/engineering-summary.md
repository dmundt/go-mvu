---
type: Engineering Summary
title: Engineering summary
description: Project direction, implementation status, and technical summary.
version: 7
---

# Engineering Summary

Direction + status. Source of truth: [gomvu spec](specs/gomvu-chat-spec.md)
([map, deviations](specs/README.md)). History:
[engineering brief](engineering-brief.md).

## Goal

Narrow Go framework for server-rendered HTML apps: MVU on server, typed
components streaming HTML, HTMX progressive enhancement.

- Platform: Go + `net/http`.
- Server owns state, validation, persistence, rendering.
- Pages and fragments distinct contracts.
- Themes: system/light/dark, CSS files inlined, adapter-agnostic.

Non-goals: SPA, hydration, virtual DOM, JSON UI protocol, app JavaScript,
templates, reflection rendering, non-HTML output.

## Flow

```text
HTTP request → middleware → route → typed Msg → Update (pure)
  → CAS persist (bounded retry) → commands (outside store)
  → View(model, RequestContext) → Page | Fragment → stream → HTMX swap
```

## Status

| Area | State |
| --- | --- |
| Pure contracts: `mvu`, `html`, `htmx`, `store` | done, tested |
| `web`: runtime, RequestContext, typed forms, CommandRunner, RenderAuto | done, tested |
| `theme`: default tokens, system mode, signed preference | done; Go + node tests |
| `components`: controls, layouts, native widgets, page policy, secure actions | done; unified gallery example |
| `web`: CSP, CSRF, stream-safe recover, diagnostics | done, tested; request/cache policy application-owned |
| `components`: local HTMX 2.0.4 + SRI | done |
| examples: counter, users, component gallery, themes | done; HTTP tests |
| boundaries test, benchmarks | done |
| race tests | not run locally (cgo off); CI with `CGO_ENABLED=1` |

## Known limits

- Memory store is process-local; the durable example demonstrates a simple
  JSON file store outside framework packages, with single-process scope.
- `style-src 'unsafe-inline'` required by inline CSS.
- Shared widgets: tooltip uses native Escape-dismissible popover; tabs are
  native navigation links, not scripted ARIA tablists.
- OIDC production example supplies authentication integration, not user
  management. Durable grants make local logout survive restart.
- Cookie revocation exists; its in-memory backend is process-local, not
  distributed infrastructure.

## Extension rules

- New abstraction only for concrete HTML/HTMX need.
- New package → allowlist in `internal/arch` + architecture doc.
- New hot-path component → benchmark.
- Default-theme CSS changes → computed-style browser checks + `npm run test:themes`.

## Related

- [Architecture](architecture.md) · [ADRs](adr/README.md) ·
  [README](../README.md)
