# Tasks

- [x] 1.1 Verify `orca worktree current --json` output: `result.worktree.linkedIssue`, number or null (confirmed live on Orca 1.4.255: an integer, e.g. 78)
- [x] 1.2 `workspace setup`: resolve from branch, then from Orca's linked issue, else exit 0 doing nothing
- [x] 1.3 `workspace setup`: pull the linked issue into a change when none exists
- [x] 1.4 `workspace setup`: set stage `active`
- [x] 1.5 `workspace archive`: resolve the same way, set stage `auto`
- [x] 1.6 Register `workspace` in the dispatcher and `agent-help`
- [x] 1.7 Tests with a fake Orca runner: branch match, linked-issue match, issue with no change, unrelated workspace
- [x] 1.8 README "Working with Orca": the `orca.yaml` recipe and the one-status-writer rule
- [x] 1.9 `site/features.json`, specsync skill
- [x] 1.10 Narrow `in-flight-work-claims` and sync it
- [x] 1.12 Find a committed change in a fresh worktree through the issue marker, binding with `adopt` rather than `pull`
- [x] 1.13 `-sync` and `-dry-run` on both commands
- [ ] 1.11 Run the recipe end to end in Orca on this repo. Setup verified live: Orca ran the `orca.yaml` setup hook and `workspace setup` found the change through issue #78 in a fresh worktree. Archive hook not yet run: Orca reads it from the primary checkout's `orca.yaml`
