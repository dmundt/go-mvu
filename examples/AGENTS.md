# Example rules

- Demonstrate public framework APIs and keep examples runnable using `go run`.
- Preserve full-page navigation alongside HTMX fragments. Use page/fragment
  writers for HTML content type and report rendering failures.
- Tables must be visibly styled by the selected theme, including plain
  semantic tables in the users example.
- Keep custom reusable components in normal Go packages, not registration
  systems. Cover imported components and handlers with tests.
- Cross-example helpers live in `examples/internal/` (`themepicker`); Go
  `internal` rule keeps them out of framework API. Reuse them; do not copy
  theme lookup or picker forms. Single-example code stays in that example.
  Never name a folder `shared` here (clashes with `shared/` package).
- Layout: `main.go` = server wiring + `newHandler`; view/components in
  separate files. Lines ≤100 cols; one field per line for multi-field
  literals that would wrap.
- `examples/components` (port 8083) is the `shared` widget gallery. Add every
  new `shared` widget there; cover it in `main_test.go`.
- Do not terminate an existing process just to reuse an example port; use an
  available port or an isolated HTTP test server.
