---
type: Architecture Decision Record
title: "ADR-0004: Stream HTML directly with explicit escaping"
description: Render HTML directly while preserving explicit safe escaping.
version: 1
---

# ADR-0004: Stream HTML directly with explicit escaping

- Status: Accepted

## Context

The framework needs a small, predictable HTML rendering path. A generic
renderer, intermediate DOM, reflection, and runtime template system add
indirection without serving the framework's HTML-only goal. Direct output
still needs clear safety boundaries for untrusted values.

## Decision

Components render HTML directly to an `io.Writer` through the `html` package.
Use explicit APIs for distinct content contexts: `Static` for
developer-controlled markup, `Text` for escaped text, and `Attribute` for
escaped attribute values. Nested components use the same rendering context,
which preserves the first write/render error.

Do not expose a general-purpose raw-string rendering path. Any trusted-HTML
escape hatch must use an explicit trusted type and accept only audited,
server-controlled markup.

## Consequences

- Rendering remains streaming and avoids building a virtual DOM.
- Call sites make text, attribute, and static-markup boundaries visible.
- User-controlled values must never be passed as static or trusted markup.
