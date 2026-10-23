---
type: Architecture Decision Record
title: "ADR-0005: Distinguish pages from fragments"
description: Define separate response contracts for full pages and fragments.
version: 1
---

# ADR-0005: Distinguish pages from fragments

- Status: Accepted

## Context

The same application serves complete browser documents and smaller HTML
subtrees for HTMX swaps. Treating both as an undifferentiated component makes
it easier to return a full document where a fragment is required.

## Decision

Model full pages and swap-safe fragments as distinct HTML contracts. A page
represents a complete HTML document; a fragment represents an HTML subtree.
The web layer exposes page and fragment response paths and selects the
appropriate projection from the view.

HTMX actions use typed attributes to describe requests, targets, and swap
behavior. The server responds with HTML, not a separate UI protocol.

## Consequences

- Page and fragment intent is visible at the type boundary.
- HTMX responses can replace targeted regions without introducing client-side
  application state.
- Applications own the mapping from incoming routes/messages to named
  fragments.
