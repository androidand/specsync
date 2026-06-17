# OpenSpec-native workflow guardrails

## Why

`specsync` already syncs OpenSpec changes to tracker issues, but the repo still
describes mostly the sync mechanics. To make dogfooding robust and consistent,
the project should explicitly follow OpenSpec lifecycle guardrails (validate,
status checks, and predictable change hygiene) as part of normal development.

Without these guardrails, we risk:

- malformed OpenSpec changes reaching sync,
- inconsistent behavior between contributors,
- treating OpenSpec as optional documentation rather than the planning source of
  truth.

## What

Define and enforce an OpenSpec-native way of working in this repository:

1. Validate OpenSpec changes before sync/push in CI.
2. Document the expected lifecycle (`propose -> tasks -> apply -> sync`).
3. Keep `specsync` resilient by continuing to read files directly when the
   OpenSpec CLI is unavailable, while optionally using OpenSpec CLI status
   checks where appropriate.

This change is process and guardrails first. It does not change provider
behavior or linking logic.

## Scope

- README guidance for OpenSpec lifecycle discipline.
- CI checks that validate OpenSpec change structure before running sync.
- Developer workflow notes for issue-first and spec-first paths.

Out of scope:

- replacing existing markdown parsing with OpenSpec CLI-only integration
- provider features (GitHub/MCP/pluggable providers)
- linker or conflict-resolution behavior