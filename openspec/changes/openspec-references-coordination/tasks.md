# Tasks: coordinate via OpenSpec references and worksets

## Read OpenSpec coordination (no new registry)
- [ ] Read `openspec context --json` → referenced stores + resolved local paths (root + referenced_store members)
- [ ] Read `openspec workset list --json` → named folder sets for local ergonomics
- [ ] Version-guard + tolerant-parse the JSON (same discipline as the OpenSpec trace adapter); degrade cleanly when absent/older

## Surface in planning output
- [ ] Add referenced siblings to `scan`/`relate`: sibling repo, local folder, its related changes/issues
- [ ] Optional `--references` view that lists just the coordination graph

## Suggest tracker edges (never auto-create)
- [ ] Where a reference implies a dependency, suggest a `## Blocked by` entry for confirmation
- [ ] Do not write any GitHub relationship here — projection stays with issue-dependency-sync / epic-and-subissue-projection

## Boundaries & tests
- [ ] Read-only; stdlib-only; shells out to `openspec`; `boundary_test.go` green
- [ ] Fake-runner tests: references surfaced, worksets surfaced, suggestion emitted, clean degradation with no references
- [ ] Update the specsync skill: how references/worksets feed the two-worktree workflow
