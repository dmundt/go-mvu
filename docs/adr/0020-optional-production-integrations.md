---
type: Architecture Decision
title: Optional production integrations
description: Keep CASK persistence and OIDC application wiring outside core framework packages.
version: 1
---

# ADR-0020: Optional production integrations

Status: Accepted

## Context

Adoption needs a runnable path from the first counter to durable authenticated
deployment. The core deliberately excludes database drivers and identity
management. CASK provides single-host content-addressed objects and atomic
references, but no transactional compare-and-swap on named references.

## Decision

- Ship `adapters/caskstore` as a separate Go module depending on go-cask.
  Implement the existing `store.Store[string, M]` interface, not an ORM.
- Serialize transitions under an adapter mutex and exclusive OS directory
  ownership. Explicitly support one owning process on local storage only.
- Store JSON records with schema/version envelopes, digest checks, and logical
  deletion. Propagate failures, including potentially committed publication
  errors. Do not invent automatic migrations, GC, or multi-model transactions.
- Ship `examples/production` as another optional module using established OIDC
  and OAuth2 libraries. Keep accounts, authentication grants, session rotation,
  logout, and HTTP authorization in application wiring, outside MVU updates.
- Persist authentication grants so local application logout survives restart;
  do not claim distributed cookie revocation or provider-global logout.
- Reuse existing rate-limit middleware and document process-local scope.
- Keep framework packages stdlib-only; root module's durable outbox example
  already uses CASK. Root module tests do not replace nested-module
  tests. No changes to rendering contracts, application JavaScript, or CSS.

## Consequences

Single-host users get a working durable example without choosing an ORM or
building an identity platform. Multi-instance users still need shared
transactional storage and appropriate identity/quota infrastructure.
Optional modules have separate Go/dependency requirements and release tags.
See [production guide](../guides/production.md) and
[adapter contract](../../adapters/caskstore/README.md).
