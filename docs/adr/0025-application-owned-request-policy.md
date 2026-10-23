---
type: Architecture Decision Record
title: Application-owned request and cache policy
description: Keep middleware focused on CSRF, security headers, streaming recovery, and diagnostics.
version: 1
---

# ADR-0025: Application-owned request and cache policy

Status: Accepted

## Decision

- Remove public `middleware.NewRequestPolicy`, `RequestPolicyOptions`, and
  `NoStore`. Body-size limits, cooperative deadlines, accepted content types,
  and cache headers are application policies built on `net/http` and `context`.
- Retain CSRF and its rendering helpers, default security headers/CSP,
  stream-aware `Recover`/`RecoverWith`, and runtime diagnostics.
- Preserve example limits and cache behavior. Shared example-server helpers
  are internal application code, not a new framework policy package.
- Keep the counter starter copyable outside the repository. Its small local
  wrapper uses a fixed body limit and cooperative timeout without exposing a
  new configurable policy abstraction.

## Consequences

Applications upgrading must replace removed wrappers before compiling.
Apply body limits before all parsers, even when CSRF arrives in a header.
Check known oversized lengths before downstream execution when required.
Readers must report size errors as 413; routes own content-type rejection.
Context deadlines cannot roll back committed state or forcibly stop handlers.
Never append timeout errors after response commitment.

Session-bearing non-runtime responses must set no-store before output. Public
static assets retain their independent cache policy; runtime no-store defaults
remain unchanged.

This supersedes only the retained request/cache wrappers in ADR-0024. Runtime
diagnostics and custom recovery responses remain supported framework features.
