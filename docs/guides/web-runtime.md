# Web runtime

`web` binds an `mvu.Application` to `net/http`: key resolution, message
decoding, versioned persistence, commands, rendering. `mvu` stays free of
HTTP, HTML, storage.

## Configuration

```go
runtime, err := web.NewRuntime(web.RuntimeConfig[string, Model, Message]{
	App:    App{},                          // required
	Store:  store.NewMemory[string, Model](), // required
	Keys:   web.KeyResolverFunc[string](sessionKey), // required
	View:   View{},                         // required
	Theme:  theme.Default,                  // nil → theme.DefaultTheme
	Themes: preference,                     // nil → theme.ModeFromContext
	Commands: web.SyncCommands[Message]{},  // nil → SyncCommands
	OnError:  web.InternalServerError,      // nil → generic 500
	Logger:   slog.Default(),               // nil → slog.Default
	MaxRetries: 3,                          // ≤ 0 → 3
})
```

- Missing required field → `ErrInvalidRuntime`. `MustRuntime` panics instead
  (startup only).
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
- Outside the runtime: `web.NewRequestContext(r, theme, resolver)`.

## Routes

```go
mux.Handle("GET /{$}", runtime.Page(web.Result{Mode: web.RenderPage}))

mux.Handle("POST /counter/increment", runtime.Handle(web.Route[Message]{
	Method:  http.MethodPost,
	Pattern: "/counter/increment",
	Decode:  web.StaticMessage[Message]{Message: Increment{}},
	Result:  web.Result{Mode: web.RenderAuto, Fragment: "counter", Location: "/"},
}))
```

- `Page(Result)`: load or create model (runs Init commands), render, no
  Update. `PageFunc(func(*http.Request) Result)` for per-request results.
- `Handle`: decode → Update → persist → commands → render.
- `DecoderFunc` for request-dependent messages (users example: `?page=N`).
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
| `AsyncCommands{OnError}` | one goroutine per command, `context.WithoutCancel`; response renders immediately; errors → `OnError` or slog |

Custom runner: implement `Run(ctx, commands, dispatch) error` (queues,
worker pools, outbox).

`AsyncCommands` is deprecated because it is unbounded. Prefer:

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
`ErrCommandClosed` signals shutdown. A batch may enqueue a prefix before
returning an error. Neither enqueue nor effects are transactional.

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

`Route.ExpectedVersion` accepts `ExpectedVersionResolver`:
`ResolveVersion(*http.Request) (store.Version, error)`.
`web.ExpectedVersionFunc` adapts functions. A stale initial action returns
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

## Middleware order

Request execution order (first wrapper outermost):

```go
var handler http.Handler = mux
policy, err := middleware.NewRequestPolicy(middleware.RequestPolicyOptions{
	MaxBodyBytes: 1 << 20,
	Timeout:      10 * time.Second,
})
handler = middleware.Chain(handler,
	middleware.RequestID,
	middleware.SecurityHeaders,
	middleware.RequestLogger(logger),
	diagnostics.Middleware, // optional
	middleware.Recover,
	policy,
	middleware.NoStore,
	sessions.Wrap,
	preference.Wrap,
	csrf.Wrap,
)
```

Session before CSRF (token bound to session). Preference before views.
Security headers on every response, including errors. Recovery inside observers
ensures panic responses are recorded with request IDs. Mount public static
assets outside session and CSRF wrappers. See [middleware](middleware.md)
and [security](security.md).

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
handler = diagnostics.Middleware(handler)                    // inside RequestID
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
  and request ID.
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
  AppShell, three-mode switcher, local HTMX, diagnostics +
  `GET /debug/diagnostics`.
- [users](../../examples/users/): `DecoderFunc` from query, clamped page in
  `Update`, fragment/page by HTMX header, `EmptyState`.
