# Security

Defaults safe; opt-out explicit.

## Escaping

- `Context.Text` / `Context.Attribute` escape; no `Raw(string)`.
- `Static` literals and `Trusted(TrustStatic(...))` = audited server HTML
  only; never request, form, DB, URL, or user input.
- `htmx` attribute values escaped; URLs validated (root-relative fast path,
  else `url.Parse`). Only relative, `http`, `https` allowed;
  else `ErrInvalidAction`.

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

`middleware.SecurityHeaders` sets on every response:

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

- `static` package embeds HTMX 2.0.4; served at `/static/` via
  `static.Handler()`.
- `static.HTMXPath` + `static.HTMXIntegrity` → `Document.HTMXIntegrity`
  emits SRI `integrity`. Version bump → update hash ([static/AGENTS.md](../../static/AGENTS.md)).

## CSRF

- `middleware.NewCSRF(secret)`: HMAC token bound to session ID.
- Methods other than GET/HEAD/OPTIONS/TRACE require form field `_csrf` or
  header `X-CSRF-Token`; else 403.
- `CSRFInput{}` renders hidden field in forms. `CSRFHTMX{Body}` adds
  `hx-headers` so HTMX requests carry header.
- Order: sessions → csrf (token needs session).

## Cookies

| Cookie | Flags |
| --- | --- |
| session | signed ID, HttpOnly, SameSite=Lax default, `Secure` option, MaxAge option |
| theme | signed mode, HttpOnly, SameSite=Lax default, `Secure` option |

- Secrets ≥ 32 random bytes; separate secrets per purpose.
- Production behind HTTPS → `Secure: true`.
- Tampered cookie → ignored (new session / `System` mode).

## Redirects

- `theme.SafeReturnPath`: only same-origin root-relative paths; `//host`,
  schemes, backslashes, control chars rejected → `/`.
- `ThemeSwitcher` drops unsafe `Return` values at render time.

## Errors and logging

- `middleware.Recover`: panic → 500, logged, no stack to client.
- `web.InternalServerError`: generic body; details only in log.
- Errors after response start: logged, never appended to body.
- `middleware.RequestID`: `X-Request-ID` per response; `RequestLogger`
  logs method, path, status, bytes, duration, request ID, htmx flag via
  `slog`.
- `middleware.Diagnostics`: `Server-Timing` off by default; enable only for
  development or trusted networks. Mount `Diagnostics.Handler()` on internal
  or authenticated routes only (timing data aids side-channel probing).

## Validation

- Decode in `web.Decoder`; reject bad input before `Update`.
- `validation` package collects field errors; render with
  `components.FieldError`.
- `Update` clamps domain values (users example clamps page).

## Known limits

- No rate limiting, auth, or session revocation store; add at HTTP boundary.
- Memory store / sessions single-process; multi-instance → shared store.
