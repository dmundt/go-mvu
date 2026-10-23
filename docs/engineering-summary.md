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
| `mvu`, `html`, `htmx`, `store`, `session`, `validation` | done, tested |
| `web` runtime: RequestContext, ThemeResolver, CommandRunner, RenderAuto | done, tested |
| `theme`: tokens, system mode, signed preference, 3 adapters | done; Go + node tests |
| `components`, `shared` | done; gallery example |
| `middleware`: CSP, CSRF, recover, request ID, logging | done, tested |
| `static`: local HTMX 2.0.4 + SRI | done |
| examples: counter, users, components, shared gallery, themes | done; HTTP tests |
| boundaries test, benchmarks | done |
| race tests | not run locally (cgo off); CI with `CGO_ENABLED=1` |

## Known limits

- Memory store + sessions single-process; durable store = app adapter.
- `style-src 'unsafe-inline'` required by inline CSS.
- Shared widgets: tooltip no Escape; tabs = links, no ARIA tablist keys.
- No auth, rate limiting, session revocation.

## Extension rules

- New abstraction only for concrete HTML/HTMX need.
- New package → allowlist in `internal/arch` + architecture doc.
- New hot-path component → benchmark.
- New theme → system variant + `npm run test:themes`.

## Related

- [Architecture](architecture.md) · [ADRs](adr/README.md) ·
  [README](../README.md)
