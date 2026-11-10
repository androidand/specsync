# Tasks

Depends on `status-pr-and-worktree-state` for `local.size` and for the attention requirement that this change modifies (apply the MODIFIED delta in `specs/pr-size/spec.md` when archiving); uses slice budgets from `plan-changes-as-slices` when present.

## 1. Analysis

- [x] 1.1 Count files, changed lines and commits against the merge base, with `-exclude <glob>`
- [x] 1.2 Parse conventional-commit scopes from commit subjects; collect distinct scopes
- [x] 1.3 Resolve the budget: flag, slice line, `SPECSYNC_BUDGET`, defaults; zero means no limit, malformed values are errors

## 2. Surface

- [x] 2.1 Add `oversized`, `mixed-scopes`, `no-slice-plan` to the attention vocabulary with their rules
- [x] 2.2 `specsync verify -size [-fail-on-oversized]` for a branch or change request
- [x] 2.3 Show the flags and the exceeded limit in `status`

## 3. Tests

- [x] 3.1 Fixture branches: within budget, over each limit, excluded paths
- [x] 3.2 Scope fixtures: two scopes, one scope, none, malformed subjects
- [x] 3.3 Budget precedence order
- [x] 3.4 Advisory exit 0 versus `-fail-on-oversized` exit non-zero
- [x] 3.5 No slice is ever generated

## 4. Docs

- [x] 4.1 Implement each new command or flag once, as a `Command` type per `make-commands-discoverable`; README, `agent-help`, the overview and the skill are generated, never hand-written
- [x] 4.2 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
