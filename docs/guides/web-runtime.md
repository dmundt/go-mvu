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

## Middleware order

Outermost first:

```go
var handler http.Handler = mux
handler = csrf.Wrap(handler)
handler = preference.Wrap(handler)
handler = sessions.Wrap(handler)
handler = middleware.RequestLogger(logger)(handler)
handler = middleware.RequestID(handler)
handler = middleware.SecurityHeaders(handler)
handler = middleware.Recover(handler)
```

Session before CSRF (token bound to session). Preference before views.
Security headers on every response, including errors. See
[security](security.md).

## Examples

- [counter](../../examples/counter/): `Page` + `Handle`, `RenderAuto`,
  AppShell, three-mode switcher, local HTMX.
- [users](../../examples/users/): `DecoderFunc` from query, clamped page in
  `Update`, fragment/page by HTMX header, `EmptyState`.
