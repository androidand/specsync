# Tasks

## 1. audit-tasks command

- [x] 1.1 Add `AuditTasks(changes []Change) TaskAuditResult` in `audit.go` — scans changes, counts unchecked/total tasks, flags mismatches where code exists but tasks are unchecked.
- [x] 1.2 Add `TaskAuditFinding` and `TaskAuditResult` types with Slug, Unchecked, Total, HasCode, CodeRefs, Progress, Stage fields.
- [x] 1.3 Add `countTasks(markdown string)` helper to parse `[ ]`/`[x]` from tasks.md.
- [x] 1.4 Add `hasImplementationEvidence(changeDir string)` — detects implementation via `.specsync/metadata.json` stage `complete`/`implemented`.
- [x] 1.5 Add `specsync audit-tasks` subcommand in `cmd/specsync/main.go` with table output.
- [x] 1.6 Add `-json` flag to `specsync audit-tasks` for machine-readable output.
- [x] 1.7 Add `-fail-on-mismatch` flag — exits non-zero when mismatches exist.
- [x] 1.8 Register `"audit-tasks": true` in `knownSubcommands` map.
- [x] 1.9 Add "audit-tasks" to help text subcommand list.

## 2. CI enforcement

- [x] 2.1 Add `specsync audit-tasks -fail-on-mismatch` CI step to `.github/workflows/ci.yml`.

## 3. Fix false positives

- [x] 3.1 Tighten `hasImplementationEvidence` — drop specs/ + design.md heuristic (design docs contain Go type stubs during planning). Rely only on metadata stage `complete`/`implemented`.

## 4. Reconcile existing drift — check off done tasks

- [x] 4.1 `reject-unknown-subcommands`: 0 remaining (task 4 is external FusionHub docs, marked owner task).
- [x] 4.2 `stable-projection-ref-key`: 0 remaining (verified by unit tests).
- [x] 4.3 `github-projects-compatibility`: 0 remaining (verified by unit tests).
- [x] 4.4 `rich-change-state`: 0 remaining (96 tasks all checked off — enums, metadata, stage derivation, tests, docs).

## 5. Reconcile existing drift — spin off follow-ups

- [x] 5.1 `two-way-reconcile`: Check off docs (8) and default-on (12); spin off 3-way merge (10) and stable task IDs (11) to `three-way-reconcile`.
- [x] 5.2 `beads-provider`: Check off docs (10); spin off auto-detection (9) to `beads-autodetect`.
- [x] 5.3 `openspec-native-workflow`: Spin off CI guardrails (2.1-2.3) to `openspec-ci-validation`.
- [x] 5.4 `launch-readiness`: Spin off owner tasks (3.1-3.4) to `launch-promotion`.

## 6. Audit and document all remaining changes

- [x] 6.1 `change-status-cli`: Check off core implementation tasks with notes; ~60 remain (sorting, TASKS/TITLE columns, diagnostics, atomic writes, tests, docs).
- [x] 6.2 `board-state-reconciliation`: Audit against code — 41 done, 19 remaining. Update tasks.md with code references. Fix design.md to match actual code (merged `MergeBase` into `BoardBinding`, added `ThreeWayDecision`).
- [x] 6.3 `skein-specsync-alignment`: 0/131 (Skein is separate codebase). Add note to tasks.md.
- [x] 6.4 Audit all remaining not-started changes: confirm no code exists for `openspec-references-coordination`, `spec-issue-linker`, `spec-source-abstraction`, `board-status-two-way`, `living-plan`, `multi-provider-sync`, `epic-and-subissue-projection`, `issue-dependency-sync`, `archive-retention-lifecycle`.

## 7. Clean up empty changes

- [x] 7.1 Remove `fix-completion-lifecycle` (empty, only `.specsync/`).
- [x] 7.2 Remove `_fit-*` directories (not OpenSpec changes, only `.skein/`).

## 8. Verification

- [x] 8.1 `go build ./...` — clean build.
- [x] 8.2 `go vet ./...` — no issues.
- [x] 8.3 `go test ./...` — all 234 tests pass.
- [x] 8.4 `specsync audit-tasks -fail-on-mismatch` — exits 0, no mismatches.
