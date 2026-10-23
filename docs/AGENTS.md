---
type: Documentation Instructions
title: Documentation rules
description: Rules for maintaining framework documentation accurately.
version: 1
---

# Documentation rules

- Follow [OKF v0.2](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md):
  every directory has an `index.md`; only root index carries `okf_version`;
  every other Markdown file starts with frontmatter containing `type`.
- Every Markdown file has an integer `version` as the last YAML frontmatter
  field. Start at `1` and increment for substantive document revisions.
- Keep indexes as navigational routers with links and short descriptions for
  every concept and child directory.
- Spec ([specs/gomvu-chat-spec.md](specs/gomvu-chat-spec.md)) authoritative;
  verbatim, never edit body. Deviations → [specs/README.md](specs/README.md).
- Engineering brief = history; never edit.
- Guides describe real public APIs only; verify names in source.
- Public behavior change → update guide, README, spec map; decision change →
  new ADR, mark superseded one.
- Benchmarks doc: real measured numbers only, with machine.
- Relative links. Telegraph style (root AGENTS.md).
