# Web runtime rules

- Views get state via `RequestContext`; no context lookups in view code.
- Commands run via `CommandRunner` after successful CAS; never inside store
  calls or `Update`.
- Never hold store state across render; bounded retry on `ErrConflict`.
- Errors before first byte → `OnError`; after → log only. Never leak error
  text to clients.
- New `RenderMode` / `Result` field → update
  [web runtime guide](../docs/guides/web-runtime.md) + tests.
- Tests use `httptest`; cover HTMX and non-HTMX paths.
