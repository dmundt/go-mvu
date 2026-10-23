# Shared component rules

- Optional widget package. Follow [ADR-0010](../docs/adr/0010-shared-ui-components.md).
- Native HTML only: `details`, `popover`, form controls, links. No scripts,
  no style attributes, no vendor class names.
- Validate IDs, URLs, variants, numeric ranges; return errors, never partial
  unsafe markup. Escape via `Text`/`Attribute`.
- Styles live in `theme/default/shared.css`. Vendor conflicts: fix in
  `theme/<adapter>/adapter.css`, then `npm run build:themes`.
- New widget: add tests in `shared_test.go`, entry in `examples/components`,
  browser check across default/Bootstrap/Bulma/Pico, light/dark, 390px.
- Spec components (`Inline`, `EmptyState`, `Checkbox`, `Textarea`) live in
  `components`; `shared/aliases.go` keeps type aliases. Never redefine them here.
