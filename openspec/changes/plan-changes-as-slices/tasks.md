# Tasks

## 1. Grammar and validation

- [x] 1.1 Parse `## Slice <n>: <title>` blocks and the `after`, `budget`, `contract` lines; attribute tasks to slices
- [x] 1.2 Keep tasks.md without slice headings byte-identical in behaviour
- [x] 1.3 Validate contiguity, `after` targets, acyclicity, orphan tasks and missing contracts in `validate` and `audit` (`validate`; `audit` compares archived changes with pull requests and has no structure checks to extend)

## 2. Commands

- [x] 2.1 `specsync slices [-change] [-json]` with progress, order, budget and derived state via the status join
- [x] 2.2 `specsync slice start <n>`: branch and worktree, stacked on the previous slice when `after` is set; record the binding
- [x] 2.3 Show a branch's slice in `status`, and use the slice budget as the threshold in the size advisory

## 3. Projection

- [x] 3.1 `sync -slices`: one child item per slice and a blocked-by per `after`, through the provider
- [x] 3.2 Fallback for providers without hierarchy: list slices in the issue body
- [x] 3.3 `-dry-run` shows exactly what would be created

## 4. Tests

- [x] 4.1 Grammar fixtures: valid plan, missing contract, cycle, orphan task, no slices
- [x] 4.2 `slice start` in a fixture repo: stacked and unstacked, binding recorded
- [x] 4.3 Projection with a mocked provider, with and without hierarchy support
- [x] 4.4 Property: specsync never emits a slice that the author did not declare

## 5. Docs

- [x] 5.1 Implement each new command or flag once, as a `Command` type per `make-commands-discoverable`; README, `agent-help`, the overview and the skill are generated, never hand-written
- [x] 5.2 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
