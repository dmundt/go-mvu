# Component rules

- Keep this library optional. Use [ADR-0009](../docs/adr/0009-reusable-components.md).
- Emit semantic, accessible HTML and framework-neutral classes, not vendor
  class names. Themes own colors, sizing, borders, and typography.
- Use `Text` and `Attribute` for dynamic content and `out.Component` for nested
  components. Keep errors sticky.
- Compose from primitives (`Heading`, `Paragraph`, `Button`, `Link`, `Output`)
  before writing tags via `Static`. Validate enum fields; return errors.
- Test page and fragment output, escaping, labels, validation associations,
  and component variants. Styling tests must include real computed styles
  when changing layout or CSS behavior.
- Spec §13 components belong here, not in `shared`. Moving one from `shared`:
  keep a type alias in `shared/aliases.go`; error prefix `components:`.
