---
type: Guide
title: HTTP middleware contracts
description: Composition, response lifecycle, request policies, and optional deployment extensions.
version: 2
---

# HTTP middleware

HTTP policies stay outside pure MVU updates. The standard library handler
signature remains the composition boundary. No registration system, reflection,
automatic middleware sorting, or application JavaScript.

## Composition and lifecycle

`middleware.Chain(handler, wrappers...)` lists request execution order:
first wrapper outermost, return path reversed. A rejection returns without
calling the next handler.

```go
policy, err := middleware.NewRequestPolicy(middleware.RequestPolicyOptions{
	MaxBodyBytes: 1 << 20,
	Timeout:      10 * time.Second,
})
if err != nil {
	return err
}
handler := middleware.Chain(mux,
	middleware.RequestID,
	middleware.SecurityHeaders,
	middleware.RequestLogger(logger),
	diagnostics.Middleware,
	middleware.Recover,
	policy,
	middleware.NoStore,
	sessions.Wrap,
	preference.Wrap,
	csrf.Wrap,
)
```

Recovery runs inside observers, with a request ID already assigned. Recovery
before final response commitment produces a safe 500. A panic after commitment
aborts through `http.ErrAbortHandler`; it never appends an error page.
The standard library abort sentinel passes through unchanged.
Logging and diagnostics finalize even during panic unwinding; diagnostics expose
an `aborted` count, and include aborted responses in `errors`.

`httpx.Writer` tracks final status, accepted body bytes, and commitment. A 1xx
other than 101 does not commit the final response. Duplicate final headers are
ignored. Use `http.NewResponseController(w)` for flush, hijack, and deadlines:
wrappers expose `Unwrap`, and flush commits every tracking layer before
forwarding. Do not rely on optional interface assertions against wrapped writers.

## Rejections

`httpx.Error{Status, Public, Location, Cause}` separates a safe response from
an internal cause. `httpx.RespondError` logs the cause and request ID, sets
`Cache-Control: no-store`, and does not expose internal error text.
Context cancellation maps to 408; deadline exceeded maps to 504.

`httpx.ErrorHandler` can be shared by `middleware.RecoverWith`,
`csrf.WithErrorHandler`, request policy, and runtime `OnError`.
Applications supply validation fragments or pages in this callback; transport
middleware does not know field layouts.

For 401 with a safe root-relative `Location`, ordinary requests navigate with
303; HTMX receives `HX-Redirect` and retains 401. CSRF rejection is 403 with
reload guidance. Never automatically replay a rejected mutation.

## Request bounds

`NewRequestPolicy` rejects negative limits. Zero body limit means 1 MiB; zero
timeout leaves the existing context deadline unchanged.

- Known oversized `Content-Length`: 413 before downstream execution.
- Unknown/chunked length: `http.MaxBytesReader` limits consumption; decoders
  must propagate read errors. This does not buffer or drain unconsumed bodies.
- Limits apply before form parsing, including header-based CSRF.
- `ContentTypes` optionally restricts unsafe requests with bodies to explicit
  media types. Parameters such as `charset` are accepted on requests; configured
  types have no parameters. Rejection: 415.
- Multipart decoding still needs an explicit application memory/file policy.
- Deadlines are cooperative. No goroutine races the handler to write a timeout;
  no timeout can roll back a transition already committed.

Configure `http.Server` header/read/idle timeouts separately. Request context
timeouts do not stop handlers or external clients that ignore cancellation.

## Cache and HTMX policy

Session-bearing HTML defaults to `no-store`. Mount static assets outside session
and CSRF wrappers; do not issue session cookies on public immutable asset
responses. Cacheable application routes require an explicit cache policy.

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

## Optional deployment extensions

Each constructor below returns `(middleware.Extension, error)`. An `Extension`
has the ordinary handler-wrapper signature. Invalid configuration returns an
error; handle it during startup. Nothing is registered or enabled implicitly.

| Constructor | Required policy |
| --- | --- |
| `TrustedProxies([]netip.Prefix)` | Explicit nonempty trust list; universal prefixes rejected |
| `AdmissionLimit(AdmissionOptions)` | `MaxConcurrent`, `RetryAfter`; immediate 503 on overload |
| `RateLimit(RateLimitOptions)` | `MaxEntries`, `Requests`, `Window`, application `Key` resolver |
| `Telemetry(TelemetryOptions)` | Fixed low-cardinality `Route`, `Complete` callback |
| `Gzip(GzipOptions)` | Explicit `Eligible` predicate; compression `Level`, optional `OnError` |
| `ResolveLocale(LocaleOptions)` | `Supported`, `Default`, optional `CookieName` and `Precedence` |
| `CORS(CORSOptions)` | Exact `Origins`, `Methods`, `Headers`; optional `Expose`, `Credentials`, `MaxAge` |
| `HSTS(HSTSOptions)` | Positive `MaxAge`, explicit `IncludeSubDomains` / `Preload` |

Admission, rate limiting, and CORS accept `ErrorHandler` for the application's
shared rejection policy.

### Proxy identity

`ClientFromContext(ctx)` returns `(ClientInfo, bool)` with `Address netip.Addr`
and `Scheme string`. Forwarded metadata is considered only when the direct peer
matches the configured proxy prefixes. X-Forwarded-For traversal is right to
left, stopping at the first untrusted hop; malformed chains are ignored.
A single validated X-Forwarded-Proto value is accepted from a trusted direct
peer. Never use raw forwarded headers as an identity or authorization source.

Apply proxy resolution before policies using resolved client information.
An IP is not a principal: authenticated rate-limit keys usually need an
application account or tenant identifier.

### Admission and rate limits

Admission has no wait queue: saturation returns 503 with `Retry-After`.
Rate limiting uses a bounded, lazily expiring fixed-window table. Exhausting a
key returns 429; filling the active-key table returns 503. Both return
`Retry-After`. Supply a stable, validated `Key func(*http.Request) (string, error)`;
avoid untrusted arbitrary header values that create new keys on every request.

The table and counters belong to this limiter instance in one process. Restart
resets quotas; replicas do not share them. Fixed windows permit bursts across
window boundaries. Use gateway or distributed enforcement for global quotas.
Behind a proxy, direct peer addresses identify the proxy unless explicitly
resolved through trusted-proxy policy. See the
[production walkthrough](production.md#4-put-https-and-abuse-controls-in-front).

### Telemetry

`TelemetryOptions.Complete func(context.Context, middleware.Completion)` runs
once on normal return, cancellation, or panic unwinding. It receives fixed
route label, request ID, status, accepted bytes, duration, panic value, and error.
The callback must not panic. Downstream panics are rethrown.

`TraceFromContext` exposes a validated W3C version-00 incoming traceparent as
`TraceContext{TraceID, ParentID, Sampled}`. This is an adapter hook, not a vendor
exporter or an automatic distributed tracing implementation. Applications
create spans and correlate runtime outcomes through their own adapter.
Do not use full paths, session IDs, or user identifiers as metric labels.

### Compression

Gzip is opt-in per request, not a global default for secret-bearing HTML.
For example, configure `Eligible` only for explicitly public routes and
`Level: gzip.DefaultCompression`.

Compression streams and supports `http.ResponseController.Flush`. It merges
`Vary: Accept-Encoding`, respects negotiation, and excludes HEAD, protocol
upgrades, bodyless statuses, pre-encoded/ranged responses, SSE, noncompressible
content, and `Cache-Control: no-transform`. Compression close failures invoke
`OnError` or default logging; a committed response cannot be replaced.

### Locale and browser deployment

Locale precedence explicitly lists `LocaleCookie` and/or `LocaleAcceptLanguage`.
Only supported locales are selected. `LocaleFromContext` is available at HTTP
boundaries; views receive resolved `RequestContext.Locale` directly.
Localization dictionaries and translation remain application responsibilities.

CORS validates exact allowlists and preflights, merges `Vary`, and never grants
wildcard credentialed access. It is normally unnecessary for same-origin SSR.
HSTS emits only for TLS or a trusted proxy's HTTPS scheme. Enable preload and
subdomain coverage only when the whole deployment can honor them.

## In-memory action idempotency

Use only on explicitly opted-in mutation routes, not global GET/page handlers.
`IdempotencyOptions` requires bounded `MaxEntries`, `TTL`, `MaxBodyBytes`,
`MaxResponseBytes`, a fixed `Route` label, and
`Scope func(*http.Request) (string, error)`.
Optional `MaxWaiters`, `WaitTimeout`, `Header`, `ReplayHeaders`, `Validate`,
and `ErrorHandler` complete the policy.

Default header: `Idempotency-Key`. Waiter count defaults to entry capacity;
wait timeout defaults to TTL. Prefer explicit smaller bounds:

```go
record, err := middleware.Idempotency(middleware.IdempotencyOptions{
	MaxEntries:       1024,
	TTL:              10 * time.Minute,
	MaxBodyBytes:     16 << 10,
	MaxResponseBytes: 64 << 10,
	MaxWaiters:       64,
	WaitTimeout:      5 * time.Second,
	Route:            "orders.create",
	Scope:            authenticatedOrderScope,
	Validate:         validateOrderInput,
	ReplayHeaders:    []string{"HX-Push-Url", "HX-Redirect"},
})
if err != nil {
	return err
}
mux.Handle("POST /orders", authenticate(csrf.Wrap(record(createOrder))))
```

Authenticate, authorize replay access, verify CSRF, route, and validate input
before reservation. `Validate func(*http.Request) error` runs before reservation
and again before duplicate replay, including after waiting. It must be read-only
and reentrant and must check current replay access; transition authorization
does not run on replay. Scope is also rechecked before exposing a duplicate.
The body is reset before mutation. Do not put current-model expected-version
checks in replay validation: those belong to the runtime transition, otherwise
a successful duplicate would fail its now-stale precondition instead of replay.

Scope action keys by principal/session and route. Fingerprints include method,
path/query, configured route, content type/encoding, Accept/Accept-Language,
all HX request headers, and bounded body. Reusing a key for a different
fingerprint returns 409. Concurrent duplicates wait within configured bounds
and can cancel independently. Pending entries never expire or get evicted while
the original handler is running; capacity stays bounded.

This wrapper deliberately buffers only its opted-in response. Streaming,
hijacking, and trailers are unsupported on these routes. Panic, unsupported
operations, and response overflow are terminal recorded failures for TTL;
`ErrIdempotencyResponseLimit` identifies overflow. Completed failures replay,
never rerun an ambiguous mutation.

Replays retain current request IDs, session cookies, security/cache/CORS/timing
metadata, not original request metadata. Default recorded response headers:
Content-Type, Content-Language, Content-Disposition, Content-Encoding, Location.
List required application/HTMX response headers explicitly in `ReplayHeaders`.

**Guarantee boundary:** single-process duplicate suppression while records
remain in memory. TTL expiry or restart allows execution again. No crash
durability, transaction with model persistence, or exactly-once external effects.
Use a durable application outbox or transactional action ledger when required.

Race/fuzz CI, replay authorization, shutdown, bounded operational counters,
durable action example, and response ordering:
[hardening](hardening.md).
