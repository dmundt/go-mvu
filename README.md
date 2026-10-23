# go-mvu

Go framework for server-rendered HTML apps: Model-View-Update on the server,
typed components streaming escaped HTML, HTMX as progressive enhancement.
No templates, no app JavaScript, no virtual DOM, no reflection rendering.
CSS lives in `.css` files, inlined into one `<style>` block.

Spec: [docs/specs](docs/specs/README.md). Docs index: [docs](docs/README.md).

## Non-goals

SPA, hydration, client state, JSON UI protocols, template engines, non-HTML
output, built-in database adapter, auth.

## Packages and boundaries

| Package | Role | Imports (in-module) |
| --- | --- | --- |
| `mvu` | `Init`, `Update`, `Command`, typed messages | none |
| `html` | `Component`, `Page`, `Fragment`, escaping `Context` | none |
| `store` | versioned snapshots, CAS, memory store | none |
| `theme` | inline CSS themes, system/light/dark, signed preference, adapters | none |
| `static` | embedded HTMX 2.0.4 + SRI | none |
| `validation` | explicit field errors | none |
| `htmx` | typed actions, request/response headers | `html` |
| `session` | signed session ID cookie | `store` |
| `middleware` | HTTP policies, CSP/CSRF, recovery, logging, diagnostics, opt-in deployment extensions | `html`, `session`, `timing`, `httpx` |
| `timing` | per-request phase trace (render time etc.) | none |
| `httpx` | shared error policy, request ID context, response tracking, Vary | none |
| `components` | layout, controls, forms, tables, pagination, switcher | `html`, `htmx`, `theme` |
| `shared` | native-HTML widgets (accordion, tabs, dialog, …) | `components`, `html`, `htmx` |
| `web` | HTTP runtime and transition policies | `mvu`, `html`, `htmx`, `store`, `theme`, `timing`, `httpx` |

Spec view:

```text
mvu        state transitions
store      model persistence
html       safe rendering
htmx       hypermedia attributes
theme      semantic visual tokens
components reusable HTML structures
web        HTTP orchestration
```

Boundaries enforced by `internal/arch` test. See
[architecture](docs/architecture.md).

## MVU lifecycle

Request lifecycle:

```text
Browser
  -> HTTP route
  -> typed message
  -> Update
  -> versioned store
  -> View
  -> Page or Fragment
  -> direct HTML stream
  -> HTMX swap
```

Detail:

```text
request → middleware → route → Decoder → typed Msg
  → Load/Init → Update (pure) → CompareAndSwap (retry ≤ MaxRetries)
  → CommandRunner (outside store) → dispatch follow-ups
  → View(model, RequestContext) → Page | Fragment → stream
```

```go
type App struct{}

func (App) Init(context.Context) (Model, []mvu.Command[Message], error) {
	return Model{}, nil, nil
}

func (App) Update(_ context.Context, model Model, msg Message) (Model, []mvu.Command[Message], error) {
	switch msg.(type) {
	case Increment:
		model.Count++
	}
	return model, nil, nil
}
```

`Update`: no HTTP, no HTML, no I/O. Effects → `mvu.Command`.

## Page vs fragment

- `html.Page` = full document (`components.Document`).
- `html.Fragment` = HTMX swap target. Distinct marker types.
- `web.RenderAuto`: HTMX → fragment; plain form post → 303 `Location`;
  else page.

## Component authoring

```go
type Greeting struct {
	html.FragmentMarker
	Name string
}

func (g Greeting) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, components.Paragraph("Hello, "+g.Name))
	return out.Err()
}
```

- `Static`: developer literals. `Text` / `Attribute`: escaped.
  `Trusted(TrustStatic(...))`: audited server HTML only.
- Sticky first error; return `out.Err()`.
- Slots = `html.Component` fields (`Header`, `Body`, `Footer`, `Children`).
- Prefer primitives (`Button`, `Heading`, `Paragraph`, `Section`, `Form`, `TableRow`)
  over raw tags. Guide: [components](docs/guides/components.md).
- No component fits? `html.El("time", html.Attrs{html.A("datetime", d)},
  html.Text(label))`: typed, escaped, allowlisted. Encapsulate it in a named
  low-level component; ordinary views stay composition-only. Direct streaming
  remains supported inside framework primitives and low-level components.
- Extra attributes on root-bearing struct components: `Attrs: html.Attrs{html.A("class",
  "wide"), html.A("hx-get", "/stats")}`. Class merges; owned names win;
  `on*`, `style`, `javascript:` rejected. ADR
  [0017](docs/adr/0017-element-builder-and-attrs.md).
- Authoring policy and scoped enforcement:
  [ADR-0018](docs/adr/0018-ui-authoring-boundaries.md).
  `Document`, `Paragraph`, `HeaderRow` and rootless helpers are explicit
  `Attrs` exceptions; see [guide](docs/guides/components.md#attribute-passthrough).

## HTMX enhancement and no-HTMX fallback

- `htmx.Action{Method, URL, Target, Swap}` → validated `hx-*` attributes.
- Wrap HTMX buttons in `components.Form{Method: FormPost, URL: ...}`;
  route `Result{Mode: RenderAuto, Fragment: "...", Location: "/"}`.
  Without JavaScript: form post → 303 → page.
- HTMX served locally: `static.Handler()` at `/static/`,
  `Document{HTMXSource: static.HTMXPath, HTMXIntegrity: static.HTMXIntegrity}`.

## Themes and cookies

- `components.Document` inlines `Theme.CSS()`; `Mode` → `data-theme`.
- Modes: `system` (default, OS via media query), `light`, `dark`.
- `theme.NewPreference(secret, opts)`: signed HttpOnly SameSite=Lax cookie.
  `POST /theme` → `preference.Handler("/")`.
- `components.ThemeSwitcher{Action, Target}`: three submit buttons, works
  without JS. `Target` set → `hx-target` + `outerHTML`; else `hx-swap="none"`.
- Optional `theme.DocumentClasser` → `class` on `<html>` per mode.
- Adapters: `bootstrap.Theme{}`, `bulma.Theme{}`, `pico.Theme{}`; same markup.
- Default design system: Open Props tokens, dense neutral UI for business and
  technical apps. CSS in `theme/default/`.
- Tokens: `--color-*`, `--space-*`, `--radius-*`, `--shadow-*`.
- Guide: [themes](docs/guides/themes.md).

## Pagination

`components.Pagination{Page, PageSize, TotalItems, BaseURL, QueryName,
Target, PushURL}`: plain links + optional HTMX GET; ≤ 1 page → nothing;
server clamps page in `Update`. Example: [users](examples/users/).

## Store concurrency

- `store.Store`: `Load`, `Create`, `CompareAndSwap(version)`, `Delete`.
- Conflict → reload + re-Update, bounded by `MaxRetries` (default 3).
- Missing model → `Init` + `Create`; `Route.RequireModel` → 404 instead.
- No lock held during commands or rendering.
- `NewMemoryWithClone` for models with reference fields.

## Commands

- `web.SyncCommands` (default): run after persist, before render.
- `web.NewAsyncCommandRunner`: bounded queue/concurrency, per-command timeout,
  explicit shutdown drain. Legacy `AsyncCommands` is deprecated.
- Commands dispatch typed follow-up messages through same transition path.
- Transition authorization runs on every CAS attempt and command follow-up.
  Optional expected-version checks reject stale intent with 409.
- Commit-aware outcome observations distinguish persistence from later
  effect/render failure. See [runtime policies](docs/guides/web-runtime.md).

## Rendering safety

- Escaping by API, no `Raw(string)`.
- URLs in HTMX actions: relative, `http`, `https` only.
- No `style=` attributes; no inline script.
- Errors after first byte logged, never leaked to body.

## CSRF and CSP

```go
sessions, err := session.New(secret, session.Options{Secure: true})
csrf, err := middleware.NewCSRF(secret)
preference, err := theme.NewPreference(themeSecret, theme.PreferenceOptions{Secure: true})

policy, err := middleware.NewRequestPolicy(middleware.RequestPolicyOptions{
	MaxBodyBytes: 1 << 20,
	Timeout:      10 * time.Second,
})
handler := middleware.Chain(mux,
	middleware.RequestID,
	middleware.SecurityHeaders,
	middleware.RequestLogger(logger),
	middleware.Recover,
	policy,
	middleware.NoStore,
	sessions.Wrap,
	preference.Wrap,
	csrf.Wrap,
)
```

- `CSRFInput{}` in forms, `CSRFHTMX{Body}` for HTMX header.
- Default CSP: `script-src 'self'`, `style-src 'self' 'unsafe-inline'`
  (inline CSS), `object-src 'none'`, `frame-ancestors 'none'`.
- Guide: [security](docs/guides/security.md).
- Composition order is explicit; recovery handles only uncommitted errors.
  Late panics abort rather than append an error page.
- `httpx.Error` and `httpx.ErrorHandler` share a rejection policy across runtime
  and middleware. Details remain in logs; HTTP error statuses remain intact.
- `Document.HTMXSwapValidationErrors` opts into 422 validation-fragment swaps.
- Mount public static assets outside session/CSRF wrappers.
- Contracts, request bounds, and extensions:
  [middleware guide](docs/guides/middleware.md).
- Session cookies now have a server-verified lifetime (default 24 hours);
  previous cookie format resets. Rotation, revocation, and signing-key rollover
  are explicit. Memory revocation/idempotency are non-durable and single-process.

## Run examples

```powershell
go run .\examples\counter      # http://localhost:8080
go run .\examples\users        # http://localhost:8081/users?page=1
go run .\examples\themes       # http://localhost:8082
go run .\examples\components   # http://localhost:8083 (shared widgets)
```

Loopback only; random per-process secrets; in-memory state.

## Deployment

- Secrets ≥ 32 random bytes from secret manager; separate per purpose.
- HTTPS + `Secure: true` cookies.
- Memory store = single process; multi-instance → shared `store.Store`
  implementation + sticky or shared sessions.
- Binary self-contained: CSS + HTMX embedded.

## Validation

```powershell
gofmt -l .
go vet ./...
go test ./...
go test '-run=^$' '-bench=.' -benchmem ./benchmarks
npm ci; npm run test:themes      # only for theme CSS changes
```

`node_modules` needed only to rebuild adapter CSS and the Open Props subset
(`npm run build:themes`).

## Benchmarks

Counter fragment 444 ns, 1 alloc vs `html/template` 645 ns, 8 allocs
(i7-13800H, single run). API minimizes framework allocations; full HTTP
request not zero-alloc.
Details: [benchmarks](docs/guides/benchmarks.md).

## Known limitations

- Race tests need cgo (`CGO_ENABLED=1 go test -race ./...`); not run on
  cgo-less dev machine.
- `style-src 'unsafe-inline'` required.
- No durable store, auth, rate limiting.
- Tooltip lacks Escape dismissal; tabs lack ARIA arrow keys.

## Extension guidelines

- Abstractions only for concrete HTML/HTMX needs.
- App components stay in app; reusable → `components` / `shared` with tests.
- New package/edge → `internal/arch` allowlist + architecture doc.
- New theme → system variant + `npm run test:themes`.
- Decisions → [ADRs](docs/adr/README.md).
