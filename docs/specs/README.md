# Specs

- [gomvu-chat-spec.md](gomvu-chat-spec.md): authoritative build spec,
  verbatim from "Go MVU Framework for SSR" chat. Original link needs
  Microsoft sign-in; this copy is source of truth.
- [engineering-brief.md](../engineering-brief.md): earlier design history.

## Section map

| § | Topic | Implementation | Docs |
| --- | --- | --- | --- |
| 1 | Product goal | whole module | [README](../../README.md) |
| 2 | Architecture, dependency direction | package layout, [internal/arch](../../internal/arch/) | [architecture](../architecture.md), ADR [0002](../adr/0002-package-boundaries.md), [0014](../adr/0014-enforced-package-boundaries.md) |
| 3 | Rendering model, markers | [html](../../html/) | ADR [0004](../adr/0004-direct-safe-html-rendering.md), [0005](../adr/0005-pages-and-fragments.md) |
| 4 | HTML context, escaping | [html](../../html/) | [security](../guides/security.md) |
| 4.1 | Element builder, attribute allowlist | `html.El`, `html.Attrs`, `htmx.Action.Attrs` | [components](../guides/components.md#low-level-element-builder), [security](../guides/security.md), ADR [0017](../adr/0017-element-builder-and-attrs.md) |
| 5 | Composition, slots | [components](../../components/) | [components](../guides/components.md) |
| 6 | MVU contracts | [mvu](../../mvu/) | ADR [0003](../adr/0003-mvu-runtime-boundaries.md) |
| 7 | Versioned store | [store](../../store/), [session](../../session/) | ADR [0006](../adr/0006-versioned-session-models.md) |
| 8 | Web views, RequestContext | [web](../../web/) | [web runtime](../guides/web-runtime.md), ADR [0013](../adr/0013-runtime-request-context.md) |
| 9 | HTMX actions, request/response | [htmx](../../htmx/) | [components](../guides/components.md) |
| 10 | HTTP runtime, lifecycle | [web](../../web/) | [web runtime](../guides/web-runtime.md) |
| 11 | Theme system, tokens, modes | [theme](../../theme/) | [themes](../guides/themes.md), ADR [0007](../adr/0007-server-controlled-themes.md), [0011](../adr/0011-system-theme-mode.md) |
| 12 | Theme switcher, `POST /theme` | `components.ThemeSwitcher`, `theme.Preference.Handler` | [themes](../guides/themes.md) |
| 13 | Component library | [components](../../components/), [shared](../../shared/) | [components](../guides/components.md), ADR [0009](../adr/0009-reusable-components.md), [0010](../adr/0010-shared-ui-components.md) |
| 13.1 | Attribute passthrough | `Attrs` field on `components`, `shared` | [components](../guides/components.md#attribute-passthrough), ADR [0017](../adr/0017-element-builder-and-attrs.md) |
| 14 | Pagination | `components.Pagination` | [components](../guides/components.md) |
| 15 | Counter example | [examples/counter](../../examples/counter/) | — |
| 16 | Users example | [examples/users](../../examples/users/) | — |
| 17 | Validation | [validation](../../validation/) | ADR [0008](../adr/0008-explicit-validation.md) |
| 18 | Middleware | [middleware](../../middleware/), [static](../../static/) | [security](../guides/security.md), ADR [0012](../adr/0012-csp-and-local-htmx.md) |
| 18.1 | Render timing, diagnostics | [timing](../../timing/), `middleware.Diagnostics`, [web](../../web/) | [web runtime](../guides/web-runtime.md#diagnostics), ADR [0016](../adr/0016-render-timing-diagnostics.md) |
| 19 | CSS | [theme/*.css](../../theme/), [scripts](../../scripts/) | [themes](../guides/themes.md) |
| 20 | Testing | `*_test.go`, `scripts/themes.test.mjs` | [README](../../README.md#validation) |
| 21 | Benchmarks | [benchmarks](../../benchmarks/) | [benchmarks](../guides/benchmarks.md) |
| 22 | Documentation | [docs](../README.md), [README](../../README.md) | — |
| 23 | Quality rules | [AGENTS.md](../../AGENTS.md) files | — |

## Deviations

| Spec | Implementation | Reason |
| --- | --- | --- |
| module `gomvu` | `github.com/dmundt/go-mvu` | repo path |
| plain theme cookie | signed cookie (`theme.Preference` = `CookieThemeResolver`) | tamper resistance |
| `NewContext(w, theme, mode)`, `Render(..., theme, mode)`, `Context.Theme()` | `NewContext(w)`, `Render(ctx, w, c)`; theme via `web.RequestContext` / `Document` | html stays theme-free (no html → theme import) |
| CSP: no unsafe-inline for script | script strict; `style-src 'unsafe-inline'` | inline CSS requirement (ADR 0012) |
| `AppShell` | `Shell` + `AppShell` alias | backward compatibility |
| `html.TrustedHTML` | + `html.Trusted` alias | brevity |
| packages per §2 | + `shared`, `static`, `internal/arch`, `benchmarks` | widgets, asset, enforcement |
| middleware deps: html only | middleware → `html`, `session`, `timing` | CSRF bound to session; shared phase trace |
| `NewRuntime` | + `MustRuntime` | startup convenience |
| pagination always rendered | hidden when ≤ 1 page | no empty nav |
| `POST /theme` only | + `Preference.ToggleHandler` | legacy two-state switches |
| users cards via runtime | plain handler + `web.NewRequestContext` | demonstrates non-runtime use |
| race tests (§25.6) | not run locally | cgo disabled on dev machine; run in CI with `CGO_ENABLED=1` |
| `Text{Value}` struct | `components.Text` string type | `Text("x")` shorter; same escaping |
| `Theme.DocumentClass`, `Theme.ComponentClass` | `Theme{Name, Stylesheet}`; optional `theme.DocumentClasser`; no `ComponentClass` | CSS variables + fixed class names; no per-component lookup on hot path |
| `RequestContext.Theme theme.Mode` | `Theme theme.Theme` + `Mode theme.Mode` | view needs theme identity and mode |
| switcher `aria-current` | `aria-pressed` on buttons | spec allows equivalent; buttons are toggles |
| pagination `BaseURL` only | + optional `URL func(page) string` | custom routes |
| `Runtime.Page(Result)` | + `PageFunc(func(*http.Request) Result)` | per-request result |
| missing model → always Init | default Init; `Route.RequireModel` → 404 | explicit init policy per route |
| `Inline`, `EmptyState`, `Checkbox`, `Textarea` in components | in `components`; `shared` keeps type aliases | backward compatibility |
| `Table` plain | `Table` wrapped in `div.table-scroll` | narrow-screen overflow |
| no generic element API | + `html.El`, `html.Attrs`, `html.Text` (§4.1) | typed low-level markup with escaping and allowlist |
| typed fields only | + `Attrs html.Attrs` passthrough on `components` and `shared` (§13.1) | extra id/class/aria/data/hx attributes without dropping to low level |
| §13.1 every component has `Attrs` | Root-bearing structs generally support it; `Document`, string/slice conveniences, rootless helpers and item records are exceptions | Preserve public APIs; [guide](../guides/components.md#attribute-passthrough) lists alternatives |
| §5 free interoperability | Retained; ordinary examples compose semantic components, two exact files demonstrate low-level APIs | Authoring discipline, not an API restriction; [ADR-0018](../adr/0018-ui-authoring-boundaries.md) |
| §13 layout inventory | + `components.Section` with `ID`, `Children`, `Attrs` | Semantic fragment roots without raw tags; layout remains in `Stack` |
