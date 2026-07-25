# Tasks

## 1. CLI

- [x] 1.1 Add `archiveCandidates` to `release-plan -json` output.
- [x] 1.2 Add `-fail-on-archive-candidates` flag to `release-plan`.
- [x] 1.3 Ensure flag exits non-zero with a clear message when candidates exist.
- [x] 1.4 Add explicit `-archive-completed` flag to execute archive moves safely.

## 2. Verification

- [x] 2.1 Add tests for `completedShipped` archive-candidate selection.
- [x] 2.2 Add tests for fail-fast decision logic.
- [x] 2.3 Add tests for archive execution helper success and destination collision.

## 3. Workflow enforcement

- [x] 3.1 Update CI workflow to run archive-hygiene guard.
- [x] 3.2 Update release workflow to run archive-hygiene guard before notes/publish.
- [x] 3.3 Update sync workflow to run archive-hygiene guard before issue projection.

## 4. Documentation

- [x] 4.1 Update README release guidance with required archive-hygiene step.
- [x] 4.2 Document explicit `-archive-completed` mutation semantics.
