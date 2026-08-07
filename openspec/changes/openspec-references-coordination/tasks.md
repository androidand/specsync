# Tasks: coordinate via OpenSpec references and worksets

## Read OpenSpec coordination (no new registry)
- [x] Read `openspec context --json` → referenced stores + resolved local paths (root + referenced_store members)
- [x] Read `openspec workset list --json` → named folder sets for local ergonomics
- [x] Version-guard + tolerant-parse the JSON (same discipline as the OpenSpec trace adapter); degrade cleanly when absent/older

## Surface in planning output
- [x] Add referenced siblings to `scan` output: sibling repo, local folder, its related changes/issues
- [x] Optional `--references` view that lists just the coordination graph
- [x] Add referenced siblings to `relate` output once `work-graph` ships the `relate` subcommand

## Suggest tracker edges (never auto-create)
- [x] Where a reference implies a dependency, suggest a `## Blocked by` entry for confirmation
- [x] Do not write any GitHub relationship here — projection stays with issue-dependency-sync / epic-and-subissue-projection

## Boundaries & tests
- [x] Read-only; stdlib-only; shells out to `openspec`; `boundary_test.go` green
- [x] Fake-runner tests: references surfaced, worksets surfaced, suggestion emitted, clean degradation with no references
- [x] Update the specsync skill: how references/worksets feed the two-worktree workflow
