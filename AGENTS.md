# Working on go-mvu

- Use dense English telegraph style in responses: concise fragments, no prose,
  fluff, or fillers. Preserve technical substance and clarity.
- Source of truth: [gomvu spec](docs/specs/gomvu-chat-spec.md); deviations in
  [docs/specs/README.md](docs/specs/README.md).
- Follow [architecture boundaries](docs/architecture.md) (enforced by
  `internal/arch` test) and accepted
  [ADRs](docs/adr/README.md). Keep MVU updates free of HTTP, rendering, and
  hidden persistence.
- Render HTML directly; do not add templates, reflection-based binding,
  virtual DOM, or application JavaScript. HTMX is the browser behavior layer.
- Design interactions as native HTML or server-rendered fragment transitions;
  keep domain/workflow state server-owned. Follow the
  [interaction guide](docs/guides/interactions.md) for fallback, accessibility,
  request ordering, and extension limits. No inline HTMX JavaScript expressions.
- Application views compose semantic components. Isolate generic HTML and
  direct streaming inside named low-level components or explicit demos;
  framework component implementations may stream directly. See ADR-0018.
- Keep styling in CSS files, embedded into document style blocks. Do not add
  external stylesheet requests or component style attributes.
- Preserve dynamic-content escaping and propagate rendering errors.
- Validate changed packages with `go test`, then `go vet` when Go code changes.
  UI changes also need computed-style or browser verification, not selector
  presence tests alone.
- Hot-path rendering change → run `go test '-run=^$' '-bench=.' -benchmem ./benchmarks`.
- Nested `AGENTS.md` files add package-specific rules. Do not edit generated
  framework CSS or vendored licenses manually.

## Git workflow (GitHub flow)

- `main` always green and releasable. No direct work on `main`.
- Per change: branch from `main` → `feat/…`, `fix/…`, `docs/…`, `refactor/…`,
  `test/…`, `chore/…` (kebab-case, short).
- Commit after each change worth committing: one logical unit, builds and
  tests green (`go vet`, `go test ./...`; `npm run test:themes` for CSS).
  No WIP or broken commits.
- Messages: Conventional Commits. Subject imperative, ≤50 chars; body = why,
  when not obvious. Include `Co-authored-by` trailer for agent commits.
- Finish: rebase on `main`, merge `--no-ff` (or PR when remote exists),
  delete branch.
- Commit `package.json`, `package-lock.json`, generated `theme/**` CSS and
  licenses. Never commit `node_modules/`, `.playwright-mcp/`, binaries,
  secrets.
- Never rewrite published history; no force-push to `main`.
