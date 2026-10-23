---
type: Architecture Decision Record
title: "ADR-0023: One reusable component package"
description: Consolidate native widgets into components without widening dependencies.
version: 2
---

# ADR-0023: One reusable component package

- Status: Accepted
- Supersedes package separation in [ADR-0010](0010-shared-ui-components.md).

Package placement and import names are superseded by
[ADR-0029](0029-seven-public-packages.md). The historical decision below
retains its behavioral contracts; use the seven-package layout and
[migration notes](../../CHANGELOG.md#unreleased) for current APIs.

## Context

`components` and `shared` both expose reusable, semantic HTML components with
the same rendering, escaping, attribute, and theme contracts. Choosing between
them adds authoring complexity without a distinct dependency boundary.

## Decision

- Move all widget implementations and tests into `components`.
- Remove `shared`, including its compatibility aliases. Consumers replace
  the import with `github.com/dmundt/go-mvu/components` and use the same
  exported widget names. Existing `Inline`, `EmptyState`, `Checkbox`, and
  `Textarea` already live there.
- Reuse existing private rendering and validation helpers where semantics
  match. Widget errors use the `components:` prefix.
- Preserve native behavior, emitted HTML, CSS classes, tokens, and embedded
  `theme/default/shared.css`. The stylesheet name is not a Go package boundary.
- Keep `components` dependent only on `html`, `htmx`, and `theme`. Optional
  application integration remains in `ui`; no HTTP or middleware dependency
  enters reusable components.

## Consequences

Applications need one reusable UI import. This is a source-breaking migration;
there is no forwarding package or second alias surface. Components remain
optional and retain their existing public properties and render contracts.
Gallery, unit, architecture, and browser tests cover the consolidated package.
