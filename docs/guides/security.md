---
type: Guide
title: Security
description: Security boundaries for HTML rendering, requests, and sessions.
version: 6
---

# Security

Defaults safe; opt-out explicit.

## Escaping

- `Context.Text` / `Context.Attribute` escape; no `Raw(string)`.
- `Static` literals and `Trusted(TrustStatic(...))` = audited server HTML
  only; never request, form, DB, URL, or user input.
- `web.BrowserAction` attribute values escaped; URLs validated (root-relative fast path,
  else `url.Parse`). Only relative, `http`, `https` allowed;
  else `web.ErrInvalidHTMXAction`.

## Attribute allowlist

`html.El`, `Context.Attrs` and component `Attrs` passthrough validate every
attribute (`html.ValidateAttr`); failures stop rendering with
`html.ErrUnsafeAttribute`:

- Names: `[A-Za-z_:][A-Za-z0-9_.:-]*`.
- Rejected: `on*` event handlers, `style` (CSS stays in stylesheets),
  `srcdoc`, `hx-on*`, `hx-vars`, `data-hx-on*`.
- `hx-vals` / `hx-headers`: no `js:` / `javascript:` evaluation.
- `hx-trigger`: no `[...]` filters (they evaluate JavaScript).
- URL attributes (`href`, `src`, `action`, `formaction`, `hx-get`, …):
  relative, `http`, `https`, `mailto`, `tel` only.
- Tags: `[a-z][a-z0-9-]*`; `script`, `style`, `link`, `meta`, `base`,
  `iframe`, `object`, `embed`, `template`, … rejected
  (`html.ErrInvalidElement`).

Values are always escaped. Allowlist does not judge semantics: an app that
passes user input as `id` or `hx-target` still owns that choice.

## Security headers

`web.SecurityHeaders` sets on every response:

| Header | Value |
| --- | --- |
| `Content-Security-Policy` | `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; form-action 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'` |
| `Permissions-Policy` | camera, microphone, geolocation, payment, usb disabled |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `X-Frame-Options` | `DENY` |

- `style-src 'unsafe-inline'`: required by inline theme `<style>`. Theme CSS =
  trusted developer content. No `style=` attributes rendered.
- `script-src 'self'`: no inline script, no CDN. App JavaScript forbidden.

## HTMX asset

- `components` package embeds HTMX 2.0.4; served at `/static/` via
  `components.AssetHandler()`.
- `components.HTMXPath` + `components.HTMXIntegrity` → `Document.HTMXIntegrity`
  emits SRI `integrity`. Version bump → update hash ([asset rules](../../components/AGENTS.md)).

## CSRF

- `web.NewCSRF(secret)`: HMAC token bound to session ID.
- Methods other than GET/HEAD/OPTIONS/TRACE require form field `_csrf` or
  header `X-CSRF-Token`; else 403.
- `components.CSRFInput{}` renders hidden field in forms. `components.CSRFHTMX{Body}` adds
  `hx-headers` so HTMX requests carry header.
- Order: sessions → csrf (token needs session).

## Cookies

| Cookie | Flags |
| --- | --- |
| session | signed ID and server expiry, HttpOnly, SameSite=Lax default, `Secure`, `MaxAge` options |
| theme | signed mode, HttpOnly, SameSite=Lax default, `Secure` option |

- Secrets ≥ 32 random bytes; separate secrets per purpose.
- Production behind HTTPS → `Secure: true`.
- Tampered cookie → ignored (new session / `System` mode).
- Session `Lifetime` defaults to 24 hours and is verified server-side;
  `MaxAge` controls only browser retention. Legacy session cookies are rejected.
- `VerificationKeys` verifies prior signing keys; only the current key signs.
- `Rotate` explicitly migrates a session-keyed model through an application
  callback. Reload the document afterward: old forms carry old CSRF tokens.
- `Invalidate` revokes copied cookies, not merely the current browser cookie.
  Its bounded memory backend is single-process and non-durable. Share
  `SessionOptions.Revocations` across middleware instances during key rollover.

## Redirects

- `theme.SafeReturnPath`: only same-origin root-relative paths; `//host`,
  schemes, backslashes, control chars rejected → `/`.
- `ThemeSwitcher` drops unsafe `Return` values at render time.

## Errors and logging

- `web.Recover`: uncommitted panic → 500; committed panic aborts.
  Logged, no stack to client.
- `web.RespondError`: safe HTTP status/body policy; details only in log.
- Errors after response start: logged, never appended to body.
- go-mvu does not ship generic request-ID or access-log middleware. Use an
  existing `net/http` middleware. To include its ID in go-mvu error and
  diagnostics logs, have it add the ID to request context with
  `web.WithRequestID`.
- `web.Diagnostics`: `Server-Timing` off by default; enable only for
  development or trusted networks. Mount `Diagnostics.Handler()` on internal
  or authenticated routes only (timing data aids side-channel probing).

## Validation

- Decode in `web.Decoder`; reject bad input before `Update`.
- `web.Form` and `web.FieldErrors` collect typed validation feedback; render with
  `components.FieldError`.
- `Update` clamps domain values (users example clamps page).

## Known limits

- Authentication and domain authorization remain application responsibilities.
  Optional transport limits do not establish identity.
- Session revocation does not survive restart or coordinate multiple processes.
  Durable effects need application persistence and an outbox or equivalent
  transaction boundary.
- Memory store / sessions single-process; multi-instance → shared store.
- Deployment policies and guarantees: [middleware](middleware.md).
