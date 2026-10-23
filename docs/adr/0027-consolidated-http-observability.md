---
type: Architecture Decision Record
title: "ADR-0027: Consolidated HTTP observability"
status: Accepted
description: Merge request phase tracing into existing HTTP support package.
version: 1
---

# ADR-0027: Consolidated HTTP observability

## Context

`timing` is a small stdlib-only package consumed by the runtime and HTTP
diagnostics. `httpx` already owns request metadata, operational events, and
response commitment. A separate package adds an import and dependency node
without a different ownership boundary.

## Decision

- Move trace implementation and tests into `httpx`.
- Name construction and lookup `NewTrace` and `TraceFromContext` so they are
  unambiguous alongside other HTTP support APIs.
- Retain `Trace`, `Phase`, `Phase*` constants, and `WithTrace` semantics.
- Delete `timing` without aliases or forwarding packages.
- Keep `httpx` stdlib-only. Runtime and middleware already depend on it;
  their dependency direction is unchanged.

## Consequences

One fewer public package. Existing applications must migrate imports and
qualifiers; see [migration notes](../../CHANGELOG.md#unreleased).
Nil-safe instrumentation, concurrent collection, phase order, aggregation,
Server-Timing headers/trailers, and streaming behavior remain unchanged.

This supersedes package placement in
[ADR-0016](0016-render-timing-diagnostics.md), not diagnostic behavior.
