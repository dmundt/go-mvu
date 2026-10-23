# Themes and CSS

Themes own presentation; components own semantic structure, accessibility, and
HTMX behavior. Framework classes are framework-neutral, so applications do not
need Bootstrap, Tailwind, or another CSS framework.

## Inline theme CSS

Default CSS = standalone files in [`theme/default/`](../../theme/default/),
not Go strings. Package `theme` embeds them at build time;
`components.Document` renders selected theme's `Stylesheet()` in one
`<style>` block. No runtime asset serving, no stylesheet request. Edit CSS,
rebuild Go app.

`Document` has no external stylesheet field; components emit no style
attributes. `Stack`/`Grid` use fixed spacing classes; `Grid` bounded column
classes.

## Default design system

Built on [Open Props](https://open-props.style) (MIT). Goal: minimal, dense,
neutral UI for business and technical apps (admin, data browsers, consoles).

| File | Role | Adapters reuse |
| --- | --- | --- |
| `open-props.css` | Generated Open Props token subset (`:where(html)`) | yes |
| `base.css` | Palette, public tokens, structure, tables, layout | yes |
| `default.css` | Design system, scoped to `html[data-theme-name="default"]` | no |
| `shared.css` | `shared` widget styles | yes |

Embed order: `open-props.css` → `base.css` → `default.css` → `shared.css`.
Package stays `theme`; `default` is a Go keyword, so `theme/default/` holds
CSS only. API unchanged: `theme.Default`, `theme.DefaultTheme`.

Design:

- Neutrals: Open Props oklch grays tinted via `--gray-hue: 250`,
  `--gray-chroma: .006`. Accent: Open Props blue.
- Density: 14px body, 13px controls/tables, 12px uppercase table headers,
  2rem control height, 4–8px radii, 1px borders, minimal shadow.
- Shell: sticky topbar, bordered sidebar, padded main. Plain pages: centered,
  max 80rem.
- Tables: bordered scroll container, sticky muted uppercase header, compact
  cells, row hover and focus-within.
- Buttons: one recipe; variants set `--button-bg/fg/border`; hover/active via
  `color-mix`.
- Theme switcher: segmented control.
- Focus: 2px solid primary outline, 2px offset.
- Light, dark, system dark palettes; reduced motion honored.

Plain `<table>` and `components.Table` get styling without extra classes.

### Open Props subset

`npm run build:themes` regenerates `theme/default/open-props.css`: scans
`var(--…)` in `base.css`, `default.css`, `shared.css`; resolves transitively
against `node_modules/open-props/src` (gray-oklch, colors, sizes, fonts,
borders, shadows, easing); emits only used tokens; copies license to
`LICENSE-open-props.txt`. No `@import`, no CDN. `open-props` pinned exact in
`package.json` devDependencies. Run after using a new Open Props token;
`npm run test:themes` fails on undefined tokens.
## Plug-and-play CSS framework adapters

Three adapters bundle real upstream framework CSS and map it to the framework's
existing component markup:

| Import | Theme | Bundled CSS |
| --- | --- | --- |
| `github.com/dmundt/go-mvu/theme/bootstrap` | `bootstrap.Theme{}` | Bootstrap 5.3.8 |
| `github.com/dmundt/go-mvu/theme/bulma` | `bulma.Theme{}` | Bulma 1.0.4 |
| `github.com/dmundt/go-mvu/theme/pico` | `pico.Theme{}` | Pico 2.1.1 |

Select an adapter by changing only `Document.Theme`:

```go
import "github.com/dmundt/go-mvu/theme/bootstrap"

doc := components.Document{
	Title: "Dashboard",
	Theme: bootstrap.Theme{},
	Mode:  theme.ModeFromContext(ctx),
	Body:  dashboard,
}
```

Use `bulma.Theme{}` or `pico.Theme{}` in the same way. No manual adapter CSS,
vendor classes, stylesheet routes, or JavaScript setup is needed. The server
still writes CSS inline. Every adapter supports `data-theme="light"`,
`data-theme="dark"`, and `data-theme="system"` and retains the shared spacing,
grid, and shell contract. The build script clones each vendor dark rule into
an `@media (prefers-color-scheme: dark)` block for `system`; rules that
negate dark (`:not([data-theme=dark])`) are skipped.
Upstream CSS classes also remain available for custom components, but their
JavaScript-dependent widgets are not included.

Try the gallery with `go run .\examples\themes`, then open
`http://localhost:8082/?theme=bootstrap&mode=dark`. It uses the same component
tree for all four themes, including an ordinary semantic table.

### Updating bundled assets

Adapter source files live in `theme/<framework>/adapter.css`. Upstream CSS
and build tools are pinned in the root `package.json` and lockfile. Maintainers
can regenerate bundled CSS with:

```powershell
npm ci --ignore-scripts
npm run build:themes
npm run test:themes
```

The build maps selectors using a CSS parser, includes shared layout overrides,
and retains upstream MIT license notices in each adapter's `LICENSE.txt`.
Generated `framework.css` files are shipped with the Go packages, so Go users
do not need Node.js or npm. Do not edit generated assets manually.

## Custom themes

A custom theme implements the small `theme.Theme` contract:

```go
type Theme interface {
	Name() string
	Stylesheet() string
}
```

Treat custom theme CSS as trusted developer-owned content. Inline CSS needs
`style-src 'unsafe-inline'`; `middleware.DefaultContentSecurityPolicy` allows
it while keeping `script-src 'self'`. See [security](security.md).

Custom themes can also keep styles outside Go source and embed the file in
their own package:

```go
package appTheme

import _ "embed"

//go:embed app.css
var css string

type Theme struct{}

func (Theme) Name() string       { return "app" }
func (Theme) Stylesheet() string { return css }
```

## Design tokens

`base.css` defines short source tokens per palette (`--bg`, `--fg`,
`--surface`, `--border`, `--primary`, `--on-primary`, `--danger`, …) and
public semantic aliases for app CSS:

| Group | Tokens |
| --- | --- |
| Color | `--color-background`, `--color-surface`, `--color-surface-raised`, `--color-text`, `--color-text-muted`, `--color-border`, `--color-primary`, `--color-primary-text`, `--color-success`, `--color-warning`, `--color-danger` |
| Focus | `--focus-ring` |
| Shadow | `--shadow-small`, `--shadow-medium` |
| Radius | `--radius-small`, `--radius-medium` |
| Space | `--space-1` … `--space-8` |

Consume `--color-*` in app CSS; set only source tokens in custom palettes.
`prefers-reduced-motion: reduce` disables transitions and animations.

## Modes: system, light, dark

| Mode | `data-theme` | Palette |
| --- | --- | --- |
| `theme.System` (zero value) | `system` | OS preference via `@media (prefers-color-scheme: dark)` |
| `theme.Light` | `light` | light |
| `theme.Dark` | `dark` | dark |

- `theme.ParseMode` strict: `system|light|dark`, else `ErrInvalidMode`.
- `theme.Parse` lenient: unknown → `System`.
- `theme.ModeFromContext(ctx)` → mode set by `Preference.Wrap`; default `System`.
- `web.RequestContext.Mode` carries the same value into runtime views.

```go
func (View) Page(_ context.Context, model Model, request web.RequestContext) (html.Page, error) {
	return components.Document{
		Title: "Dashboard",
		Theme: request.Theme,
		Mode:  request.Mode,
		Body: middleware.CSRFHTMX{Body: components.AppShell{
			Header: components.ThemeSwitcher{
				Mode:      request.Mode,
				Return:    request.CurrentURL,
				CSRFField: middleware.CSRFInput{},
			},
			Main: dashboard(model),
		}},
	}, nil
}
```

## Theme cookie and switcher

`theme.NewPreference(secret, options)` → signed, HttpOnly, SameSite=Lax
cookie, separate from session cookie. Secret ≥ 32 random bytes. `Secure: true`
behind HTTPS. Tampered or missing cookie → `System`.

`*theme.Preference` implements `web.ThemeResolver` (`Resolve`, `Persist`).
Pass it as `RuntimeConfig.Themes`.

```go
preference, err := theme.NewPreference(secret, theme.PreferenceOptions{Secure: true})
if err != nil {
	return err
}
mux.Handle("POST /theme", preference.Handler("/"))
handler := sessions.Wrap(preference.Wrap(csrf.Wrap(mux)))
```

`Preference.Handler(fallback)`:

| Input | Result |
| --- | --- |
| `mode` invalid / missing | 400 |
| HTMX request | cookie set, 204 + `HX-Refresh: true` |
| plain form | cookie set, 303 → `return` if same-origin path, else `fallback` |

`components.ThemeSwitcher` renders one POST form with three submit buttons
(`name="mode"`, values `system|light|dark`), `aria-pressed` on the active
mode, optional CSRF field and hidden `return` path (unsafe values dropped).
Works without JavaScript; HTMX enhances with `hx-post` + `hx-swap="none"`.
`Action` overrides the post URL (default `/theme`). `Target` set →
`hx-target` + `hx-swap="outerHTML"` (handler returns fresh markup).

Optional `theme.DocumentClasser` (`DocumentClass(mode) string`) adds
`class` on `<html>` for frameworks keyed on classes; resolve via
`theme.DocumentClass(t, mode)`.
`ToggleHandler` (light↔dark) remains for legacy single-button switches.

Working example: [counter](../../examples/counter/view.go).