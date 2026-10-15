# Tasks

Depends on `status-pr-and-worktree-state` for `local.size`; uses slice budgets from `plan-changes-as-slices` when present.

## 1. Analysis

- [ ] 1.1 Count files, changed lines and commits against the merge base, with `-exclude <glob>`
- [ ] 1.2 Parse conventional-commit scopes from commit subjects; collect distinct scopes
- [ ] 1.3 Resolve the budget: flag, slice line, `SPECSYNC_BUDGET`, defaults

## 2. Surface

- [ ] 2.1 Add `oversized`, `mixed-scopes`, `no-slice-plan` to the attention vocabulary with their rules
- [ ] 2.2 `specsync verify -size [-fail-on-oversized]` for a branch or change request
- [ ] 2.3 Show the flags and the exceeded limit in `status`

## 3. Tests

- [ ] 3.1 Fixture branches: within budget, over each limit, excluded paths
- [ ] 3.2 Scope fixtures: two scopes, one scope, none, malformed subjects
- [ ] 3.3 Budget precedence order
- [ ] 3.4 Advisory exit 0 versus `-fail-on-oversized` exit non-zero
- [ ] 3.5 No slice is ever generated

## 4. Docs

- [ ] 4.1 Implement each new command or flag once, as a `Command` type per `make-commands-discoverable`; README, `agent-help`, the overview and the skill are generated, never hand-written
- [ ] 4.2 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
