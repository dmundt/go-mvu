# Example rules

- Demonstrate public framework APIs and keep examples runnable using `go run`.
- Ordinary example files compose `components`, `shared`, and application
  components. Context calls: `Component`, `Components`, `Err`, `Fail` only;
  no `html.El`, `Element`, `ComponentFunc`, raw/trusted output, or writers.
- Explicit low-level demos live only in `themes/lowlevel.go` and
  `components/lowlevel.go`, declared in `internal/arch/authoring_test.go`.
  Do not add an exception to avoid composing an existing semantic component.
- Prefer typed properties over `Attrs`; use `Attrs` for genuine extensions,
  not to reimplement layout classes or override owned names.
- Keep deep composition readable with domain components (`CounterControls`,
  `UserResults`) and local values. Do not invent a generic composition DSL.
- UI changes: run `npm run test:browser`; servers bind loopback ports
  18080–18084. `browser_test.go` harnesses are opt-in and skipped by ordinary
  `go test`; never expose test-server controls in production handlers.
- Preserve full-page navigation alongside HTMX fragments. Use page/fragment
  writers for HTML content type and report rendering failures.
- Tables must be visibly styled by the selected theme, including plain
  semantic tables in the users example.
- Keep custom reusable components in normal Go packages, not registration
  systems. Cover imported components and handlers with tests.
- Cross-example helpers live in `examples/internal/` (`modepicker`); Go
  `internal` rule keeps them out of framework API. Reuse them; do not duplicate
  mode-picker forms. Single-example code stays in that example.
  Never name a folder `shared` here (clashes with `shared/` package).
- Layout: `main.go` = server wiring + `newHandler`; view/components in
  separate files. Lines ≤100 cols; one field per line for multi-field
  literals that would wrap.
- `examples/components` (port 8083) is the `shared` widget gallery. Add every
  new `shared` widget there; cover it in `main_test.go`.
- Do not terminate an existing process just to reuse an example port; use an
  available port or an isolated HTTP test server.
