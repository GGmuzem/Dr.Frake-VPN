# Domain Docs

How the engineering skills should consume this repo's domain documentation when
exploring the codebase.

## Before exploring, read these

- `CONTEXT.md` at the repo root.
- `docs/adr/` for ADRs that touch the area about to be changed.

If any of these files do not exist, proceed silently. Do not flag their absence
or suggest creating them upfront. Producer skills can create them later when
terms or decisions actually get resolved.

## Layout

This is a single-context repo:

```text
/
|-- CONTEXT.md
|-- docs/adr/
|-- client/
|-- service/
|-- vpn-backend/
`-- web/
```

## Use the glossary's vocabulary

When output names a domain concept in an issue title, refactor proposal,
hypothesis, or test name, use the term as defined in `CONTEXT.md`.

If the concept is missing, either reconsider the language or note the gap for a
future domain-doc update.

## Flag ADR conflicts

If output contradicts an existing ADR, surface it explicitly rather than
silently overriding it.
