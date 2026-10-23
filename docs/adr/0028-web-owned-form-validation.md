---
type: Architecture Decision Record
title: "ADR-0028: Web-owned form validation"
status: Accepted
description: Consolidate explicit request form decoding and field feedback into web.
version: 1
---

# ADR-0028: Web-owned form validation

## Context

The small validation package owns request decoders and typed form feedback.
Its consumers already use the HTTP runtime or the UI integration layer.
Separate placement adds a public package without a distinct runtime owner.

## Decision

- Move form validation contracts and tests into `web`.
- Keep `Form[T]`, `Bind`, and `ErrMissingDecoder`.
- Name the field-message map `FieldErrors` and the typed input decoder
  `FormDecoder[T]`, distinct from the runtime message `Decoder[Msg]`.
- Delete `validation` without aliases or forwarding packages.
- Keep parsing, normalization, validation rules, and UI composition explicit.
  `mvu` remains independent of HTTP, rendering, and storage. Application
  models may carry typed form data; Update still performs no HTTP work.

## Consequences

One fewer public package. Applications migrate imports and names; see
[migration notes](../../CHANGELOG.md#unreleased).
Field-error accumulation, first-error lookup, malformed-request propagation,
and dirty/valid form flags retain their behavior. The nil-decoder sentinel
is now `web.ErrMissingDecoder`, with a web-prefixed message.

This supersedes package placement in
[ADR-0008](0008-explicit-validation.md) and form-decoder naming in
[ADR-0026](0026-prototype-api-simplification.md), not explicit validation.
