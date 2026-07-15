# Enforce release archive hygiene

## Why

A completed, shipped change can remain in `openspec/changes/` if nobody performs
archive follow-up. That creates planning drift: active lists include finished
work, and release follow-up becomes unreliable.

This happened with `spec-driven-changelog`: feature shipped, but tasks and
archive state were stale. The failure was not projection logic; it was missing
lifecycle enforcement.

## Release note

Add machine-enforced archive hygiene: release follow-up now reports archive
candidates in structured output, supports a fail-fast guard for CI, and the
workflows enforce that no shipped-complete changes remain unarchived.

## What Changes

- Extend `specsync release-plan -json` to include `archiveCandidates` so CI can
  evaluate archive hygiene without brittle text parsing.
- Add `specsync release-plan -fail-on-archive-candidates` to exit non-zero when
  shipped + completed changes are still unarchived.
- Add explicit `specsync release-plan -archive-completed` execution to move
  shipped + completed changes into `openspec/changes/archive/` safely.
- Add tests for archive-candidate detection and fail-fast behavior.
- Enforce archive hygiene in CI/release workflows before publish steps.
- Enforce archive hygiene in the specs->issues sync workflow so stale completed
  changes fail fast before backlog projection.
- Update README release guidance to include archive-hygiene checks and archive
  follow-up as required lifecycle steps.

## Out of scope

- Automatic archiving/mutation from `release-plan`.
- Changing OpenSpec semantics for what counts as complete.

## Capabilities

- `archive-hygiene-enforcement`: detect and enforce closure of shipped completed
  changes.
