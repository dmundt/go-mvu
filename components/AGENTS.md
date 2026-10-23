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
