# Linker cleanup: LinkerResult, slug-matching, dead code removal

## Why

The linker from the spec-issue-linker change needed hardening and cleanup after
the initial merge. The branch resolver resolved all changes to the same issue
when syncing multiple changes, MarkerResolver was too expensive, and
ExternalResolver was dead placeholder code.

## What

- Add `LinkerResult` struct with `Source` field (e.g. "branch", "cache") for
  dry-run visibility.
- Fix BranchResolver slug-matching: `feat/42-my-change` only resolves when
  syncing `my-change`, preventing all-changes-to-same-issue.
- Remove MarkerResolver (API call per change, never needed in practice).
- Remove ExternalResolver (dead placeholder code).
- Simplify Pull to use BranchResolver directly instead of the full Linker chain.

## Release note

Linker resolves issue refs with source visibility (branch, cache) and slug-aware
branch matching to prevent cross-change resolution.
