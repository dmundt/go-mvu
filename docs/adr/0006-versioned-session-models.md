---
type: Architecture Decision Record
title: "ADR-0006: Keep versioned models behind storage and session boundaries"
description: Persist versioned models through storage and session boundaries.
version: 3
---

# ADR-0006: Keep versioned models behind storage and session boundaries

- Status: Accepted

Package placement and import names are superseded by
[ADR-0029](0029-seven-public-packages.md). The historical decision below
retains its behavioral contracts; use the seven-package layout and
[migration notes](../../CHANGELOG.md#unreleased) for current APIs.


The MVU/store/HTMX package split is further superseded by
[ADR-0030](0030-five-public-packages.md): MVU and storage now share `core`,
and HTMX transport lives in `web`. Behavioral contracts remain unchanged.

## Context

Server-side MVU applications need to retain model state across requests and
isolate state between sessions. The application update contract should not
depend on a particular storage engine or session mechanism.

## Decision

Keep model persistence in the `store` package and session scoping in the
`session` package. Store model snapshots with version information so the web
runtime can detect and handle concurrent updates instead of silently
overwriting newer state. Session middleware provides an opaque session
identity; storage adapters own the mapping from that identity to model state.

The framework's in-memory store is suitable for development and tests.
Applications choose durable storage appropriate to their deployment.

## Consequences

- `mvu` remains independent of persistence and sessions.
- Concurrent request handling must respect store versions.
- Restarting an in-memory deployment loses its state; durable persistence is
  an application/deployment choice.
