---
type: Architecture Decision Record
title: "ADR-0003: Keep MVU updates separate from I/O"
description: Keep state updates pure and isolate side effects in commands.
version: 1
---

# ADR-0003: Keep MVU updates separate from I/O

- Status: Accepted

## Context

HTTP handling, persistence, and rendering are environmental concerns.
Embedding them in state transitions would couple application logic to a
specific transport and make transitions difficult to test.

## Decision

Keep the MVU application contract focused on initialization and state
transitions. `Update` receives the current model and typed message, returns the
next model and commands, and does not inspect HTTP requests, render HTML, or
perform hidden persistence.

Views project a model into a page or fragment without mutating the model.
Commands represent external side effects and are executed by the runtime
outside the update function.

## Consequences

- State transitions can be tested without an HTTP server or renderer.
- The web runtime coordinates message decoding, model persistence, commands,
  view selection, and response rendering.
- Side effects are explicit in the application contract.
