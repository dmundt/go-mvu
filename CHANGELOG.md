# Changelog

Notable changes to go-mvu are documented here.

## Unreleased

### Added

- Architecture regression test requiring exactly seven public framework packages.

- `ui.ModePicker` for native GET theme previews, with explicit hidden query
  fields. Promoted from the example-only helper; gallery behavior is unchanged.

### Changed

- **Breaking:** seven public framework packages: `mvu`, `store`, `html`, `htmx`,
  `theme`, `web`, and `components`. Internal packages and examples are not part
  of this count. No forwarding packages or compatibility aliases remain.
  - `httpx`, `session`, and `middleware` move into `web`.
  - `session.New`, `Options`, `Middleware`, `ID`, and `DefaultLifetime` become
    `web.NewSession`, `SessionOptions`, `Session`, `SessionID`, and
    `DefaultSessionLifetime`. `ErrWeakSecret` and `ErrInvalidOptions` become
    `ErrWeakSessionSecret` and `ErrInvalidSessionOptions`.
  - `httpx.Writer` becomes `web.ResponseWriter`; other HTTP infrastructure,
    session lifecycle, and middleware names keep their spelling under `web`.
  - `ui` and `static` move into `components`. `ui.Document`, `ui.Form`, and
    `ui.ThemeSwitcher` become `components.PagePolicy`, `ActionForm`, and
    `SecureThemeSwitcher`. Explicit `Document`, `Form`, and `ThemeSwitcher`
    retain their low-level contracts.
  - CSRF renderers move from `middleware` to `components`: `CSRFInput` and
    `CSRFHTMX`. Token lookup and server protection stay in `web`.
  - `static.Handler` and `static.Prefix` become `components.AssetHandler` and
    `AssetPrefix`; HTMX URL, bytes, SRI, license, and caching stay unchanged.
  - Replace old imports, remove duplicate imports, and use the new qualifiers.
    UI helper error strings now start with `components:`.
  - Session cookies, CSRF signatures, markup, persistence, and HTTP behavior
    are unchanged. `web` never imports `components`; rendering remains optional.
  See [ADR-0029](docs/adr/0029-seven-public-packages.md).

- **Breaking:** merge `validation` into `web`; remove the old package without
  aliases. Replace `validation.Form` with `web.Form`, `validation.Errors`
  with `web.FieldErrors`, and `validation.Decoder` with `web.FormDecoder`.
  Use `web.Bind` and `web.ErrMissingDecoder`. The sentinel message now starts
  with `web:`; typed feedback and explicit validation behavior are unchanged.

- **Breaking:** merge `timing` into `httpx`; remove the old package without
  forwarding aliases. Replace `timing.New()` with `httpx.NewTrace()` and
  `timing.FromContext(ctx)` with `httpx.TraceFromContext(ctx)`. Move `Trace`,
  `Phase`, `WithTrace`, and `Phase*` constant qualifiers to `httpx`.
  Trace collection and diagnostics output are unchanged.

- **Breaking:** request contracts are functions:
  - `web.DecoderFunc` becomes `web.Decoder`; `KeyResolverFunc` becomes
    `KeyResolver`; `ExpectedVersionFunc` becomes `ExpectedVersion`.
  - Assign functions directly to `Decode`, `Keys`, and `ExpectedVersion`.
    Existing struct implementations pass `decoder.Decode`,
    `resolver.Resolve`, or `resolver.ResolveVersion` method values.
  - Replace `web.StaticMessage[Msg]{Message: value}` with
    `web.StaticMessage[Msg](value)`; invoke decoders as functions.
  - `validation.DecoderFunc` becomes function type `validation.Decoder`.
    `Bind` accepts functions directly and rejects nil with `ErrMissingDecoder`.
- **Breaking:** theme mode comes only from `theme.Preference.Wrap` or
  `theme.WithMode`. Remove `RuntimeConfig.Themes`; `web.ThemeResolver` is gone.
  `web.NewRequestContext` takes only request and active theme. Install preference
  middleware to retain cookie-driven appearance; custom mode resolution belongs
  in application middleware. Without it, mode defaults to System.
- **Breaking:** remove `web.MustRuntime`, `web.ErrorHandler`, and
  `web.InternalServerError`. Use error-returning `web.NewRuntime`,
  `httpx.ErrorHandler`, and `httpx.RespondError`.
- **Breaking:** remove deprecated unbounded `web.AsyncCommands`. Use
  `NewAsyncCommandRunner`, configure queue/concurrency/timeout explicitly, and
  drain it with `Close` during shutdown.
- **Breaking:** remove compatibility aliases. Replace `components.AppShell`
  with `components.Shell`, `NavigationItem` with `NavItem`, `BreadcrumbItem`
  with `Breadcrumb`, and `html.Trusted` with `html.TrustedHTML`.

- Separate runtime, rendering, controls, and example startup responsibilities
  into focused source files without changing package paths or public APIs.

- **Breaking:** consolidate `shared` widgets into `components`. Replace
  `github.com/dmundt/go-mvu/shared` imports and `shared.` qualifiers with
  `github.com/dmundt/go-mvu/components` and `components.`. Remove duplicate
  imports where both packages were used. No forwarding aliases remain.
- Widget validation errors now use the `components:` prefix. Markup, native
  interactions, CSS classes, and embedded widget styles remain unchanged.

### Removed

- Generic middleware duplicated by the `net/http` ecosystem: handler chaining,
  request IDs, access logging, CORS, HSTS, admission limits, and rate limits.
- The `middleware.Extension` API and bounded in-memory idempotency/replay
  middleware; use application-owned durable action ledgers when required.
- `middleware.NewRequestPolicy`, `RequestPolicyOptions`, and `NoStore`;
  applications own body limits, deadlines, content-type checks, and cache
  headers. Examples retain these protections using application-owned code.

Applications can compose existing middleware directly with go-mvu handlers.
Framework-specific CSRF, stream-aware recovery, security headers, request
bounds, and diagnostics remain available. Use application-owned durable action
ledgers or outboxes when mutation replay guarantees are required.

## [0.1.0] - 2026-10-23

Initial release.

### Added

- Go-first, server-owned MVU runtime for directly rendered, escaped HTML with
  HTMX progressive enhancement.
- Typed components, pages, fragments, validation, middleware policies, themes,
  and reusable UI components.
- Explicit HTTP reliability policies, request diagnostics, timing, and
  middleware deployment extensions.
- Example-only stdlib JSON file persistence and atomic outbox example.
- Production OIDC example with secure sessions.
- Browser-tested UI behaviors, theme contract checks, and HTTP benchmarks.
- Architecture, security, production, component, theme, and first-app guides.
