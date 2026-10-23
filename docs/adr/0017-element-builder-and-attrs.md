# ADR-0017: Element builder and attribute passthrough

- Status: Accepted
- Extends: [ADR-0004](0004-direct-safe-html-rendering.md),
  [ADR-0009](0009-reusable-components.md)

## Context

Two gaps between high-level components and low-level `html.Context` code:

1. Low-level markup meant hand-written tag strings via `Static`.
2. Components exposed only typed fields. Adding one `id`, `class`, `aria-*`,
   `data-*` or `hx-*` attribute meant rewriting the component by hand.

## Decision

- `html.El(tag, html.Attrs{...}, children...)`: typed element builder.
  `Attrs` is an ordered slice (deterministic output; no map, no reflection).
  `html.A`, `html.Flag`, `html.Text` helpers.
- One allowlist (`html.ValidateAttr`, `html.ValidateTag`,
  `html.ValidateURL`) for builder and passthrough. Rejects event handlers,
  `style`, `srcdoc`, htmx JavaScript evaluation (`hx-on*`, `hx-vars`,
  `js:` values, trigger filters), unsafe URL schemes, active tags.
- Root-bearing struct components generally get `Attrs html.Attrs`.
  `Document`, string/slice conveniences and rootless helpers are exceptions;
  see [guide](../guides/components.md#attribute-passthrough).
  `class` merges; component-owned names win and are skipped.
- Other duplicate names use first-value-wins (case-insensitive); emit one
  attribute. `hx-*` / `data-hx-*` aliases share identity and ownership.
  All class values merge in order. Discarded owned/duplicate values are not
  emitted or validated. This matches browser first-value handling without
  generating duplicate attributes.
- `Context.Class`, `ExtraClass`, `PassAttrs` let custom components do the
  same. `htmx.Action.Attrs()` bridges actions to `El`.
- `components` URL validation delegates to `html.ValidateURL`, which gains a
  root-relative fast path (no `url.Parse` allocation).

## Consequences

- High-level and low-level code interoperate without dropping to raw strings.
  [ADR-0018](0018-ui-authoring-boundaries.md) defines preferred authoring
  separation without removing interoperability.
- Silent skip of owned names can hide a mistake (`type` on `Button`);
  documented, tested.
- Allowlist is syntactic. It blocks script execution vectors, not
  application logic errors.
- Empty Attrs: no measurable hot-path cost (benchmarks at parity;
  Dashboard −2 allocs from the URL fast path).