# Plan a large change as reviewable slices

## Why

A change that grows during implementation becomes one pull request that nobody can
review. A real example: one change ended as a single PR with 106 files, about 4,000
changed lines and 72 commits, mixing a refactor, an unrelated feature, tests,
translations and the actual behaviour change. It was too large to review with
confidence, and its parts depended on each other so it could not be merged in pieces
either. Splitting it afterwards meant rewriting history.

The decomposition had to happen before the code, and specsync is where planning
already lives. Today `tasks.md` can hold phases, and `spinoff` can peel off *emergent*
work, but there is no way to declare, up front, that a change is **several
independently reviewable slices**, state each slice's contract and size budget, record
which depends on which, and then see each slice's branch, worktree and PR as work
proceeds. Without that, the answer to "how do we break this up" is rediscovered by
hand every time, and is usually found too late.

## What Changes

- **Slice blocks in `tasks.md`.** A heading `## Slice <n>: <title>` starts a slice.
  Optional lines under it declare `after: <n>[, <m>]` (ordering or stacking),
  `budget: files<=15, lines<=400` and `contract: <one sentence>`. Task checkboxes
  below belong to the slice. A change with no slice headings behaves exactly as
  today.
- **`specsync slices`** lists a change's slices with task progress, order, budget and
  derived state (not started, branch exists, PR open, merged), using the status join for the branch and PR.
- **`specsync slice start <n>`** creates the slice's branch and worktree, reusing the
  existing worktree conventions. When `after` is set, the branch is based on the
  previous slice's branch, so slices form a stack. It records the binding; it creates
  no PR.
- **Projection.** `sync -slices` projects each slice as a sub-issue of the change's
  issue and each `after` as a blocked-by dependency, reusing sub-issue projection and
  issue-dependency sync. Off by default, because sub-issues per slice is noise for
  small changes.
- **Validation.** `validate` and `audit` check that slice numbers are contiguous,
  `after` targets exist and are acyclic, every task belongs to exactly one slice, and
  every slice states a contract.
- **Status.** `specsync status` shows the slice a branch belongs to, and a slice's
  budget becomes the threshold used by `pr-size-advisory`.

## Non-goals

- **No automatic splitting.** Slice boundaries are authored. specsync never proposes
  or infers them, which keeps the asserted-graph principle intact.
- No PR creation, no merge or restack automation. Stack maintenance tools exist;
  specsync records the order and reports state.
- Does not replace `spinoff`: that handles work discovered mid-change; slices are a
  planned partition of one change.

## Relationship to other changes

- Uses `status-pr-and-worktree-state` for slice state and `pr-size-advisory` for
  budgets.
- Uses `epic-and-subissue-projection` and `issue-dependency-sync` for projection;
  adds no new tracker mechanism.
- Agent orchestration (for example a fan-out skill that launches one agent per part)
  can call `slice start` per slice. specsync does not orchestrate.

## Impact

- `tasks.md` grammar extension (backwards compatible); a slice parser; two
  subcommands (`slices`, `slice start`); one projection flag; validation rules.
- Docs: README, `agent-help`, the skill, and `site/features.json`.

## Release note

Plan a large change as slices in `tasks.md`, each with a contract, an order and a size
budget, and see each slice's branch, worktree and pull request with `specsync slices`.
