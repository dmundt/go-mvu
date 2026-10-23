---
type: Architecture Decision
title: Optional production integrations
description: Keep example JSON persistence and OIDC application wiring outside core framework packages.
version: 1
---

# ADR-0020: Optional production integrations

Status: Accepted

## Context

Adoption needs a runnable path from the first counter to durable authenticated
deployment. The core deliberately excludes database drivers and identity
management. The durable example needs persistence for restart demonstrations
without adding a storage library to the framework.

## Decision

- Keep the JSON file store inside the durable example, outside framework
  packages. Implement the existing `store.Store[string, M]` interface without
  an external storage library.
- Serialize transitions within one process and explicitly document that
  multiple processes must not write the same data directory.
- Store one versioned JSON record per model key, using a temporary file and
  rename for publication. Do not invent automatic migrations, retention, or
  multi-model transactions.
- Ship `examples/production` as another optional module using established OIDC
  and OAuth2 libraries. Keep accounts, authentication grants, session rotation,
  logout, and HTTP authorization in application wiring, outside MVU updates.
- Persist authentication grants so local application logout survives restart;
  do not claim distributed cookie revocation or provider-global logout.
- Reuse existing rate-limit middleware and document process-local scope.
- Keep framework and durable-example storage code stdlib-only. Root module
  tests do not replace nested-module tests. No changes to rendering contracts,
  application JavaScript, or CSS.

## Consequences

Users get a working durable example without choosing an ORM or building an
identity platform. Multi-instance users still need shared transactional
storage and appropriate identity/quota infrastructure.
The OIDC example has separate Go/dependency requirements and release tags.
See [production guide](../guides/production.md) and
[durable example](../../examples/durable/README.md).
