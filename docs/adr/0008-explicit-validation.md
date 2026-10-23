# ADR-0008: Decode and validate forms explicitly

- Status: Accepted

## Context

Form decoding and validation are application-specific, while reflection-based
binding hides input conversion and validation behavior. The framework needs to
support server-side form feedback without imposing an implicit schema system.

## Decision

Use explicit request decoders that return typed values, field-level validation
errors, and decoding errors. Keep validation errors and form state in the
`validation` package. Do not use reflection-based form binding.

## Consequences

- Applications control parsing, normalization, and validation rules.
- Invalid input can be rendered back through ordinary HTML components.
- Decoder and validation behavior is directly testable without hidden
  reflection conventions.

