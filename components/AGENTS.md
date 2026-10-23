# Component rules

- Keep this library optional. Use [ADR-0009](../docs/adr/0009-reusable-components.md).
- Emit semantic, accessible HTML and framework-neutral classes, not vendor
  class names. Themes own colors, sizing, borders, and typography.
- Use `Text` and `Attribute` for dynamic content and `out.Component` for nested
  components. Keep errors sticky.
- Compose from primitives (`Heading`, `Paragraph`, `Button`, `Link`, `Output`)
  before writing tags via `Static`. Validate enum fields; return errors.
- Root-bearing struct components expose `Attrs html.Attrs` (controls: on the
  control). Exceptions: `Document` owns document metadata; string/slice
  conveniences (`Paragraph`, `HeaderRow`) retain their API. `Text`, `Empty`,
  `Sequence` have no root; option/navigation item structs are data, not
  components. See guide for alternatives. Merge class via
  `out.ExtraClass`/`out.Class`; write the rest
  with `out.PassAttrs(attrs, owned...)` listing every name the component
  writes (`when(field, "id")` for optional ones). See ADR-0017.
- Direct streaming is normal inside semantic component implementations;
  do not force primitives through `html.El`. Reuse existing components when
  semantics match. See ADR-0018.
- Test page and fragment output, escaping, labels, validation associations,
  and component variants. Styling tests must include real computed styles
  when changing layout or CSS behavior.
- All reusable framework widgets belong here, including native overlays,
  display widgets, and form controls. No separate `shared` package or aliases.
  See ADR-0023. Widget errors use prefix `components:`.
- Native interactions use `details`, `popover`, controls, and links; no scripts.
  Validate IDs, URLs, variants, and ranges before emitting unsafe markup.
- Widget styles remain in `theme/default/shared.css`; preserve CSS classes
  and tokens during package moves. Add gallery entries and browser checks
  for new widgets; retain computed-style coverage in light/dark at 390px.
- Application integration helpers may import `web`; `web` must never import
  `components`. Keep HTTP middleware and session state out of renderers.
- `htmx.min.js` is the vendored HTMX 2.0.4 upstream build. Never edit it.
  Version changes replace the file, recompute `HTMXIntegrity` (SHA-384, base64),
  update documentation, and run `go test ./components ./examples/...`.
- Preserve `htmx.LICENSE`. Assets use `embed`; no CDN or runtime fetch.
  Mount `AssetHandler()` at `AssetPrefix` without another `StripPrefix`.
  Directory listing returns 404; assets cache immutable for one year.
  Changing asset content requires a new filename or accepting stale clients.
