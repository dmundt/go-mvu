# ADR-0012: Strict CSP with locally served HTMX

- Status: Accepted

## Context

CDN script needs third-party `script-src`. Inline theme CSS needs inline
styles. App JavaScript forbidden.

## Decision

- Embed HTMX 2.0.4 in `static`; serve under `/static/` with immutable cache.
- `Document.HTMXIntegrity` emits SRI hash.
- `middleware.SecurityHeaders` default CSP: `script-src 'self'`,
  `style-src 'self' 'unsafe-inline'`, `object-src 'none'`,
  `frame-ancestors 'none'`, `form-action 'self'`, `base-uri 'self'`.
- No `style=` attributes, no inline script, no event handler attributes.

## Consequences

- No external runtime dependency; offline/air-gapped deploys work.
- `'unsafe-inline'` styles accepted trade-off; theme CSS trusted only.
- Nonce/hash CSP for styles possible later at HTTP boundary.
- HTMX upgrade = file replace + hash update ([static/AGENTS.md](../../static/AGENTS.md)).
