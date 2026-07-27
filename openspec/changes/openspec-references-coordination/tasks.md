# Tasks: coordinate via OpenSpec references and worksets

## Read OpenSpec coordination (no new registry)
- [ ] Read `openspec context --json` → referenced stores + resolved local paths (root + referenced_store members)
- [ ] Read `openspec workset list --json` → named folder sets for local ergonomics
- [ ] Version-guard + tolerant-parse the JSON (same discipline as the OpenSpec trace adapter); degrade cleanly when absent/older

## Surface in planning output
- [ ] Add referenced siblings to `scan` output: sibling repo, local folder, its related changes/issues
- [ ] Optional `--references` view that lists just the coordination graph
- [ ] Add referenced siblings to `relate` output once `work-graph` ships the `relate` subcommand

## Suggest tracker edges (never auto-create)
- [ ] Where a reference implies a dependency, suggest a `## Blocked by` entry for confirmation
- [ ] Do not write any GitHub relationship here — projection stays with issue-dependency-sync / epic-and-subissue-projection

## Agent/worktree handoff and merge readiness
- [ ] Add a read-only `handoff`/`coordination` report joining OpenSpec task
  ownership, references, worksets, synced issue/provider identity, current
  branch/worktree, base/current revisions, dirty/unpushed state, related changes,
  and directed dependencies.
- [ ] Report duplicate active worktrees/branches and overlapping claimed paths;
  distinguish warnings from blocking conditions and never delete or lock work.
- [ ] Add a deterministic pre-merge report flagging stale bases, missing PRs,
  unpushed commits, unverified completed tasks, and unresolved dependencies. It
  MUST show the exact remediation command or next action.
- [ ] Add an explicit-provider check so related changes synced through different
  providers produce an actionable mismatch rather than an opaque missing-ref
  error.
- [ ] Add a post-merge cleanup report for branch deletion, worktree removal,
  OpenSpec archival, and issue closure; mutations remain opt-in.
- [ ] Define stable JSON output for handoff/readiness/cleanup for local agents
  and automation.

## Boundaries & tests
- [ ] Read-only; stdlib-only; shells out to `openspec`; `boundary_test.go` green
- [ ] Fake-runner tests: references surfaced, worksets surfaced, suggestion emitted, clean degradation with no references
- [ ] Fake-runner tests: duplicate worktree, stale base, provider mismatch,
  overlapping ownership, missing PR, unpushed commits, merge-order dependency,
  and post-merge cleanup recommendations
- [ ] Update the specsync skill: how references/worksets feed the two-worktree workflow
