---
type: Guide
title: HTTP middleware contracts
description: Standard net/http composition, response lifecycle, and framework-specific HTTP policies.
version: 8
---

# HTTP middleware

HTTP policies stay outside pure MVU updates. The standard library handler
signature remains the composition boundary. No registration system, reflection,
automatic middleware sorting, or application JavaScript.

Framework middleware is limited to CSRF and its rendering helpers, default
security headers, streaming-aware recovery, and runtime diagnostics.
Request-size limits, deadlines, accepted content types, and cache policy belong
to the application. See [ADR-0025](../adr/0025-application-owned-request-policy.md).

## Composition and lifecycle

go-mvu handlers and middleware use standard `net/http` interfaces. Compose
wrappers directly or use the router's middleware stack. A rejection returns
without calling the next handler.

```go
router := chi.NewRouter()
router.Use(existingMiddleware, middleware.SecurityHeaders, middleware.Recover)
router.Handle("/", http.MaxBytesHandler(mux, 1<<20))
```

Recovery before final response commitment produces a safe 500. A panic after
commitment aborts through `http.ErrAbortHandler`; it never appends an error
page. The standard library abort sentinel passes through unchanged. If an
external request-ID middleware adds its ID to context, recovery and diagnostics
include it in their logs.
Diagnostics finalize even during panic unwinding; diagnostics expose an
`aborted` count, and include aborted responses in `errors`.

`httpx.Writer` tracks final status, accepted body bytes, and commitment. A 1xx
other than 101 does not commit the final response. Duplicate final headers are
ignored. Use `http.NewResponseController(w)` for flush, hijack, and deadlines:
wrappers expose `Unwrap`, and flush commits every tracking layer before
forwarding. Do not rely on optional interface assertions against wrapped writers.

## Existing middleware and routers

Runtime endpoints (`Runtime.Handle`, `Runtime.Page`, `Runtime.PageFunc`) and
middleware use standard `net/http` contracts. Mount them directly in
`http.ServeMux`, chi, or another router accepting `http.Handler`; use that
router's middleware stack where route grouping or framework-specific behavior
is useful. Existing middleware with signature `func(http.Handler) http.Handler`
composes directly:

```go
app := runtime.Handle(route)
router := chi.NewRouter()
router.Use(existingMiddleware, anotherMiddleware)
router.Handle("/", app)
```

The router example uses chi only as an illustration; go-mvu does not depend on
it or require a particular router. For middleware with a different API, use
its documented bridge to produce an `http.Handler` or
`func(http.Handler) http.Handler`; keep that adapter in the application. No
framework-specific integration package or middleware composer is needed.

Preserve request context and response streaming. Middleware that buffers or
replaces `http.ResponseWriter` must preserve `http.ResponseController`
operations, especially `Flush`, and should expose `Unwrap` when wrapping the
writer. Verify compatibility before placing such middleware around streamed
HTML responses.

## Rejections

`httpx.Error{Status, Public, Location, Cause}` separates a safe response from
an internal cause. `httpx.RespondError` logs the cause and request ID, sets
`Cache-Control: no-store`, and does not expose internal error text.
Context cancellation maps to 408; deadline exceeded maps to 504.

`httpx.ErrorHandler` can be shared by `middleware.RecoverWith`,
`csrf.WithErrorHandler`, application request policy, and runtime `OnError`.
Applications supply validation fragments or pages in this callback; transport
middleware does not know field layouts.

For 401 with a safe root-relative `Location`, ordinary requests navigate with
303; HTMX receives `HX-Redirect` and retains 401. CSRF rejection is 403 with
reload guidance. Never automatically replay a rejected mutation.

## Request bounds

Use `http.MaxBytesHandler` or `http.MaxBytesReader` before any parser, including
CSRF. Applications own configured limits and rejection responses; no framework
`NewRequestPolicy`, `RequestPolicyOptions`, or `NoStore` API is provided.

- Check known oversized `Content-Length` explicitly when 413 must precede
  downstream execution; `MaxBytesHandler` alone only limits reads.
- Unknown/chunked length: `http.MaxBytesReader` limits consumption; decoders
  must propagate read errors and map `*http.MaxBytesError` to 413. This does not
  buffer or drain unconsumed bodies.
- Limits apply before form parsing, including header-based CSRF.
- Routes can check `mime.ParseMediaType` against accepted body formats and
  reject unsupported types with 415.
- Multipart decoding still needs an explicit application memory/file policy.
- Deadlines are cooperative. No goroutine races the handler to write a timeout;
  no timeout can roll back a transition already committed.

Configure `http.Server` header/read/idle timeouts separately. Request context
timeouts do not stop handlers or external clients that ignore cancellation.
Use `context.WithTimeout`, always cancel it, and do not append timeout errors
after response commitment. The copyable
[counter request limits](../../examples/counter/policy.go) demonstrate this.
Other repository examples share
[application-owned helpers](../../internal/example/server/policy.go); these are
example code, not framework APIs.

## Cache and HTMX policy

Session-bearing HTML defaults to `no-store`. Mount static assets outside session
and CSRF wrappers; do not issue session cookies on public immutable asset
responses. Cacheable application routes require an explicit cache policy.
For non-runtime routes, set `Cache-Control: no-store` in application handlers
before writing; runtime rendering already defaults to no-store.

`httpx.Vary` merges case-insensitively without replacing existing values.
Representation-dependent responses must vary on all relevant inputs, including
HTMX request, boosted/history mode, locale, and theme where applicable.
Do not share session-bound CSRF markup or use model version alone as an HTML ETag.

`components.Document{HTMXSwapValidationErrors: true}` emits an escaped
`htmx-config` meta element before the HTMX script. It enables swapping 422
validation responses, keeps 204 bodyless, and leaves other 4xx/5xx unswapped.
Opt in only when 422 contains the correct validation fragment for its target.
Ordinary requests still need a full-page error response. No application
JavaScript or inline script is introduced.

See [runtime policies](web-runtime.md) and [security](security.md).

## Session lifecycle

`session.New(secret, Options)` keeps `Wrap` and `ID`. Cookie format now includes
a signed, server-verified expiry; legacy cookies start a new session.
`Lifetime` defaults to 24 hours. `MaxAge` is browser retention only and cannot
extend the signed lifetime. `Clock` supports deterministic expiry tests.

```go
revocations, err := session.NewMemoryRevocations(10000)
if err != nil {
	return err
}
sessions, err := session.New(currentKey, session.Options{
	Secure:           true,
	Lifetime:         24 * time.Hour,
	VerificationKeys: [][]byte{oldKey},
	Revocations:      revocations,
})
```

Old keys verify only; no automatic renewal or re-signing occurs.
Remove old keys only after their intended verification window.

`sessions.Rotate(w, r, migrate)` returns `session.Rotation{OldID, NewID}`.
The callback `func(oldID, newID string) error` explicitly migrates the
application's model key. It must leave application state unchanged on error,
must not write the response, and must not reenter session lifecycle methods.
Callback failure retains the old session and issues no replacement.
Successful rotation revokes the old cookie and begins a fresh lifetime.
Reload afterward to obtain CSRF tokens bound to the new session.

`sessions.Invalidate(w, r) error` revokes the old signed cookie and deletes
the browser cookie. Applications explicitly delete associated model data when
required. Both methods return errors: handle them before writing a response.

`MemoryRevocations` is bounded, cleans expired entries on access, and returns
an error rather than silently dropping revocations when full. Default capacity:
10000. Default backend belongs to one middleware instance; share an explicit
backend across instances during key rollover. It is concrete, not a distributed
backend interface. Restart loses revocations; already-admitted requests are
not canceled. Do not claim durable logout or cross-process revocation.

## Application and deployment policies

go-mvu does not ship CORS, HSTS, admission, or rate-limit middleware. Use an
existing middleware package, router integration, or deployment edge for these
policies. CORS is usually unnecessary for same-origin SSR. Apply HSTS at the
TLS terminator when TLS ends at a proxy. Use distributed enforcement for
global quotas; never trust arbitrary forwarding headers for client identity.

## Mutation retries and durable effects

go-mvu does not buffer and replay mutation responses or provide a generic
idempotency middleware. Use an application-owned durable action ledger or
transactional inbox/outbox when duplicate delivery must not repeat effects.
Ordinary model persistence and command execution are not one transaction;
see the [durable store and outbox guide](durable-store-outbox.md) and
[hardening](hardening.md).
