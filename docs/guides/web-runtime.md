---
type: Guide
title: Web runtime
description: Runtime configuration, routes, request context, and rendering.
version: 7
---

# Web runtime

`web` binds an `mvu.Application` to `net/http`: key resolution, message
decoding, versioned persistence, commands, rendering. `mvu` stays free of
HTTP, HTML, storage.

## Ownership and guarantees

The runtime owns the request-driven server loop, not a desktop event loop:

```text
HTTP request -> key + decoder -> typed message -> pure Update
             -> versioned save -> commands -> latest model -> View -> HTML
```

Applications configure routes, decoding, storage, views, and response policy
once. `Runtime.Handle` orchestrates each action; application handlers do not
need to repeat load/update/save/render logic. State lives in a keyed
`store.Store`, not a single mutable runtime field. There is no public
`Runtime.Dispatch` or `Runtime.Model` API.

| Boundary | Contract |
| --- | --- |
| Session and access | Middleware supplies session identity and protects HTTP access; `Keys` resolves the model key. Runtime does not install cookie, authentication, CSRF, or asset middleware. |
| Update | May run again after a CAS conflict. Keep it pure and preserve previous snapshots; no database writes, HTTP calls, timers, or mutation of shared slices/maps. |
| Commands | Run after persistence. Model commit, command admission, and external effects are not one transaction and are not automatically exactly-once. Use application-owned outbox/inbox or idempotency where required. |
| Rendering | Happens according to `Result`, not automatically after every background state change. Page reads can render without `Update`; redirects and `RenderNothing` skip the view. |
| Browser | HTMX requests and swaps HTML; runtime neither patches DOM nor manages focus, popover visibility, or client-local state. Plain links/forms retain full-page behavior. |
| Async completion | Follow-up messages re-enter state transitions, but do not rewrite an already-delivered response or refresh open pages. Use another request, polling, or a separately designed push integration. |
| Subscriptions | No built-in desktop-style subscription or SSE lifecycle. Commands are not an implicit streaming transport. |
| Failures | A command/render/HTTP failure after model persistence does not roll back the commit. Report failure explicitly; do not blindly replay consequential mutations. |

The [counter tutorial](first-app.md#what-runs-after-a-click) shows the smallest
wiring. The [admin example](../../examples/admin/) adds validated editing,
non-modal popover content, and coordinated page/fragment responses.

## Configuration

```go
runtime, err := web.NewRuntime(web.RuntimeConfig[string, Model, Message]{
	App:    App{},                          // required
	Store:  store.NewMemory[string, Model](), // required
	Keys:   sessionKey,                     // required
	View:   View{},                         // required
	Theme:  theme.Default,                  // nil → theme.DefaultTheme
	Commands: web.SyncCommands[Message]{},  // nil → SyncCommands
	OnError:  httpx.RespondError,            // nil → shared HTTP error policy
	Logger:   slog.Default(),               // nil → slog.Default
	MaxRetries: 3,                          // ≤ 0 → 3
})
```

- Missing required field → `ErrInvalidRuntime`. Handle startup errors explicitly.
- `Keys`, route `Decode`, and `ExpectedVersion` accept functions directly.
  To reuse a struct-based implementation, pass its method value.
- `OnError` never receives partially written responses; errors after the
  first byte are logged.

## Views and RequestContext

```go
type View[Model any] interface {
	Page(context.Context, Model, web.RequestContext) (html.Page, error)
	Fragment(context.Context, string, Model, web.RequestContext) (html.Fragment, error)
}

type RequestContext struct {
	Theme      theme.Theme
	Mode       theme.Mode
	Locale     string
	IsHTMX     bool
	HTMX       htmx.Request
	CurrentURL string // HX-Current-URL for HTMX, else request URI
}
```

- Views get presentation state explicitly; no context lookups in view code.
- `ViewFunctions` adapts plain functions. Unknown fragment →
  `ErrUnknownFragment`.
- Mode comes only from `theme.WithMode` or `theme.Preference.Wrap`; there is
  no second cookie lookup or resolver fallback in the runtime.
- Outside the runtime: `web.NewRequestContext(r, theme)`.

## Routes

For endpoints that appear in a form or button, define method, path, decoder,
and response once. Reuse the action in the view:

```go
var submit = web.Action[Message]{
	Method: htmx.Post,
	Path:   "/contact",
	Decode: decodeContact,
	Result: web.Result{Mode: web.RenderAuto, Fragment: "contact-form"},
}

if err := runtime.Register(mux, submit); err != nil {
	return nil, err
}

ui.ActionButton{Action: submit.HTMX(), Label: "Send"}
```

`HTMX()` uses `#contact-form` and `outerHTML` by default. Plain POSTs redirect
to `/contact`; set `Result.Location` to choose another destination. Set
`Target` or `Swap` to override the browser behavior. `Register` checks the
action and mounts its exact path and method.

For multi-field forms, put `submit.HTMX()` on `ui.Form`, not only on its
submit button. This keeps keyboard and button submissions consistent.
See [forms](forms.md) and [shared page policy](components.md#shared-page-policy).

Static-message actions can omit the method and decoder wrapper:

```go
result := web.Result{Mode: web.RenderAuto, Fragment: "counter", Location: "/"}
increment := web.PostMessage[Message]("/counter/increment", Increment{}, result)
reset := web.PostMessage[Message]("/counter/reset", Reset{}, result)
if err := runtime.RegisterAll(mux, increment, reset); err != nil {
	return nil, err
}
```

`PostMessage` returns a normal configurable `Action`; its response policy
remains explicit. `RegisterAll` registers in order and stops at the first
error without undoing earlier registrations. Use it during initialization,
not to reconfigure a running mux. Like `Register`, overlapping ServeMux
patterns follow the standard library's panic behavior.

Use `Route` directly when an endpoint has different routing needs or must not
share its definition with a UI action:

```go
mux.Handle("GET /{$}", runtime.Page(web.Result{Mode: web.RenderPage}))

mux.Handle("POST /counter/increment", runtime.Handle(web.Route[Message]{
	Method:  http.MethodPost,
	Pattern: "/counter/increment",
	Decode:  web.StaticMessage[Message](Increment{}),
	Result:  web.Result{Mode: web.RenderAuto, Fragment: "counter", Location: "/"},
}))
```

- `Page(Result)`: load or create model (runs Init commands), render, no
  Update. `PageFunc(func(*http.Request) Result)` for per-request results.
- `Handle`: decode → Update → persist → commands → render.
- Supply a decoder function for request-dependent messages (users: `?page=N`).
  Parse HTTP in decoders, never in `Update`.
- `Pattern` / `Method` mismatch → 404 / 405 with `Allow`.
- `RequireModel: true` → missing model = 404, no `Init` (edit routes on
  existing records).

## Result modes

| Mode | Behavior |
| --- | --- |
| `RenderPage` | full document |
| `RenderFragment` | named fragment |
| `RenderNothing` | 204 |
| `RenderAuto` | HTMX (not boosted, not history restore) → fragment; else `Location` set → 303 redirect; else page |

`Location` redirects (default 303) for non-Auto modes too. `PushURL` /
`ReplaceURL` → `HX-Push-Url` / `HX-Replace-Url`. `Status` overrides 200.
Content type always `text/html; charset=utf-8`.
`Result.CacheControl` explicitly overrides default `no-store`. Use only for
content without session-bound tokens or other private state, and account for
all representation inputs in `Vary` and validators.

`RenderAuto` + `Location` = no-JavaScript fallback: plain form posts
redirect (Post/Redirect/Get); HTMX gets the fragment.

## Lifecycle

```text
resolve key → decode message
  → [RequireModel: Load, ErrNotFound → 404]
  → Load (ErrNotFound → Init → Create; ErrAlreadyExists → retry)
  → Update(model, msg) → CompareAndSwap(version)
      ErrConflict → reload, Update again (≤ MaxRetries) → else ErrConflict
  → CommandRunner.Run(commands, dispatch)   // outside store calls
      dispatch(msg) → same transition → follow-up commands
  → Load latest → View → stream HTML
```

- No store lock held during Update result use, commands, or rendering.
- Update must stay pure: no HTTP, no I/O. Effects → `mvu.Command`.

## Commands

| Runner | Semantics |
| --- | --- |
| `SyncCommands` (default) | in order, before render; error → `OnError`; response reflects follow-ups |
| `AsyncCommandRunner` | bounded workers/queue; request values retained without cancellation; errors → callback or slog; explicit shutdown drain |

Custom runner: implement `Run(ctx, commands, dispatch) error` (queues,
worker pools, outbox).

For asynchronous work, create the bounded runner:

```go
commands, err := web.NewAsyncCommandRunner[Message](
	4, 32, 10*time.Second, reportCommandError,
)
if err != nil {
	return err
}
// Set RuntimeConfig.Commands to commands.
// During shutdown, call commands.Close(shutdownContext) and handle its error.
```

Concurrency must be positive, queue size nonnegative, and timeout positive;
invalid configuration returns `ErrInvalidCommandRunnerConfig`.
Jobs retain request context values (principal/correlation) but not request
cancellation, and receive their own timeout. Follow-up dispatch runs the same
transition authorization policy. Enqueue never waits on a worker:
`ErrCommandOverload` signals a full queue, including recursive dispatch;
`ErrCommandClosed` signals shutdown. The bounded runner reserves capacity
for each whole batch and rejects oversized batches before enqueueing;
custom runners must document their own partial-acceptance behavior.
Neither enqueue nor effects are transactional with model persistence.

`Close(ctx)` stops acceptance and drains accepted work. Deadline expiry returns
`ctx.Err()` without canceling accepted jobs; commands still need to honor their
own deadlines. Asynchronous errors go to the configured callback, not an
already-completed HTTP response.

## Transition policy and outcomes

`RuntimeConfig.AuthorizeTransition` has signature:

```go
func(context.Context, Key, Model, Message) error
```

It runs before every Update/CAS attempt, including command-dispatched
follow-ups and Handle-created initialization. Return `&httpx.Error{Status: 403}`
for rejection. Never use HX request metadata as proof of identity.
`Page`/`PageFunc` initialization remains ungated by this transition hook:
protect private page access at the HTTP boundary.

`Route.ExpectedVersion` accepts a function:
`func(*http.Request) (store.Version, error)`.
A stale initial action returns
409 without rebasing the user's intent. If CAS conflicts while this explicit
precondition is active, reject rather than retry against a newer model.
Without it, internal CAS conflicts retain bounded retries. Follow-ups do not
inherit the original action's expected version.

`RuntimeConfig.ObserveOutcome func(*http.Request, web.RequestOutcome)` runs
once for `Handle`, `Page`, and `PageFunc`, including panic unwinding. Panics
continue to HTTP recovery. The observer must not panic.

`RequestOutcome{State, Committed, Err}` separates response/effect outcome from
whether Create or CAS persisted:

| State | Meaning |
| --- | --- |
| `OutcomeRejected` | Action did not complete; inspect `Committed` for initialization persistence |
| `OutcomeCompleted` | Existing-model page completed without persistence |
| `OutcomeCommitted` | Persistence succeeded; inspect `Err` for a later pre-response failure |
| `OutcomeEffectFailed` | Persistence succeeded, then command/effect dispatch failed |
| `OutcomeResponseAborted` | Rendering or panic interrupted response; inspect independent commit truth |

`EffectError{Cause}` unwraps a failure after persistence. Default HTTP response
is 500 even for a canceled effect; this does not imply rollback.
After response commitment, render errors are logged, not appended to HTML.
Use outcome observations for audit/telemetry; never retry a mutation solely
because its HTTP response failed.

Request cancellation is checked at runtime boundaries. Wrapped
`*http.MaxBytesError` from decoding maps to 413. `RequestContext.Locale` carries
the locale selected by HTTP middleware; views need no context lookup.

See [hardening](hardening.md) for shutdown integration, transactional action
ledger example, current replay authorization, operational signals, and
document-local versus cross-tab response ordering.

## Middleware order

Request execution order (first wrapper outermost):

```go
var handler http.Handler = mux
private := sessions.Wrap(preference.Wrap(csrf.Wrap(handler)))
handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	private.ServeHTTP(w, r)
})
handler = middleware.SecurityHeaders(
	diagnostics.Middleware(
		middleware.Recover(requestLimits(handler, 10*time.Second)),
	),
)
```

`requestLimits` is application code from the
[counter starter](../../examples/counter/policy.go), not a middleware API.
It bounds body reads before CSRF, checks known oversized bodies, and uses a
cooperative context deadline without replacing committed responses.

Session before CSRF (token bound to session). Preference before views.
Security headers on every response, including errors. Recovery inside
diagnostics ensures panic responses are recorded. Mount public static assets
outside session and CSRF wrappers. Add existing request-ID and access-log
middleware through the router or standard handler composition. See
[middleware](middleware.md) and [security](security.md).

## Diagnostics

Opt-in render timing and request diagnostics. Spec §18.1,
[ADR-0016](../adr/0016-render-timing-diagnostics.md).

```go
diagnostics := middleware.NewDiagnostics(middleware.DiagnosticsOptions{
	Logger:        logger,                // nil → slog.Default()
	SlowThreshold: 50 * time.Millisecond, // Warn log above
	ServerTiming:  true,                  // dev / trusted networks only
})
mux.Handle("GET /debug/diagnostics", diagnostics.Handler()) // internal only
handler = diagnostics.Middleware(handler)                    // wrap around application handler
```

| Phase | Measures |
| --- | --- |
| `decode` | `Decoder.Decode` |
| `dispatch` | Update + store CAS + sync commands |
| `load` | store load, Init |
| `view` | `View.Page`/`Fragment` |
| `render` | HTML streaming |
| `ttfb`, `total` | added by middleware |

- Per request: slog `request diagnostics` (Debug; Warn when slow) with phases
  and optional request ID from context.
- Aggregates: count/total/mean/max per phase; requests, slow, 5xx.
  `Snapshot()`, `Reset()`, JSON `Handler()`.
- `Server-Timing` header: phases done before first byte + `ttfb`. Trailer:
  `render` + `total` (streaming → render ends after headers). Browser
  devtools show the header; trailers need chunked HTTP/1.1 or HTTP/2 and
  client support.
- Custom phases: `defer timing.FromContext(ctx).Start("db")()`.
- No trace in context → no-op, zero allocations.

## Examples

- [counter](../../examples/counter/): `Page` + `Handle`, `RenderAuto`,
  Shell, three-mode switcher, local HTMX, diagnostics +
  `GET /debug/diagnostics`.
- [users](../../examples/users/): decoder function from query, clamped page in
  `Update`, fragment/page by HTMX header, `EmptyState`.
