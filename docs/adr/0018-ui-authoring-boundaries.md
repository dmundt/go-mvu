# ADR-0018: UI authoring boundaries

- Status: Accepted
- Extends: [ADR-0009](0009-reusable-components.md),
  [ADR-0014](0014-enforced-package-boundaries.md),
  [ADR-0017](0017-element-builder-and-attrs.md)

## Context

Semantic components, generic elements and direct streaming share one
`html.Component` contract. Interoperability is useful; incidental mixing in
ordinary examples obscures ownership of markup, classes and accessibility.
Import boundaries alone cannot distinguish authoring levels.

## Decision

- Application views compose semantic components and named application
  components. `Section` supplies a semantic fragment root; `Stack` owns layout.
- Generic elements and direct streaming belong in named low-level components,
  explicit demonstrations or measured hot paths. Framework component
  implementations may stream directly; no compulsory element-builder layer.
- All levels retain `html.Component`; no separate rendering interfaces,
  virtual DOM, new UI language or package relocation.
- Repository example policy is stricter than the public API: production example
  files may use only `Component`, `Components`, `Err`, `Fail` on `html.Context`.
  Generic element construction, direct/trusted output, writer access and
  `ComponentFunc` adapters are confined to two explicit demo files. A type-aware
  test detects aliases, method references and dot imports. Ordinary application
  components in these examples compose existing semantic components too.
- Low-level exceptions are exact files with documented reasons, not filename
  patterns. Third-party applications need not adopt these repository tests.
  Scope tests guard that distinction; test files and browser harnesses are
  excluded from authoring checks. Browser regressions execute real example
  handlers and computed styles across default-theme modes and desktop/mobile.
- Package import tests discover production files recursively and require
  explicit declarations for new packages, including nested packages.
- Prefer typed properties over `Attrs`. Document actual exceptions:
  `Document`, string/slice conveniences and rootless helpers retain their API;
  item configuration records are not components.
- Shared widget triggers compose existing `Button`; specialized markup remains
  direct where existing semantics do not fit (for example icon-only close
  button content).

## Consequences

- Ordinary examples teach one default authoring style; low-level capability
  remains visible in isolated demos.
- Application views no longer own framework layout classes. Some composition
  refactors add semantic wrappers; browser checks cover layout and behavior.
- Architecture tests need Go dependency export data (`go list -export -deps`);
  no new tool or module dependency.
- Checks enforce repository conventions, not a security sandbox or universal
  proof against all indirect I/O.
