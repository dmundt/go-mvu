---
type: Architecture Decision Record
title: "ADR-0014: Enforce package boundaries by test"
description: Verify package dependency constraints with automated tests.
version: 1
---

# ADR-0014: Enforce package boundaries by test

- Status: Accepted
- Extends: [ADR-0002](0002-package-boundaries.md)

## Context

Documented dependency rules drift without enforcement.

## Decision

- `internal/arch` test parses imports of non-test files in every library
  package (`go/parser`, imports only).
- In-module imports checked against allowlist per package.
- Forbidden: `unsafe`; `reflect` in `html`, `components`; template engines
  (`html/template`, `text/template`). Tests and `benchmarks` exempt.
- Exception recorded: `middleware` → `html`, `session`.

## Consequences

- Boundary violation fails `go test ./...`.
- New package or edge → update allowlist + [architecture](../architecture.md).
