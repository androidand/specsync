# Tasks

- [ ] 1.1 Verify `orca worktree current --json` output on a real Orca workspace: field name and shape of the linked issue
- [ ] 1.2 `workspace setup`: resolve from branch, then from Orca's linked issue, else exit 0 doing nothing
- [ ] 1.3 `workspace setup`: pull the linked issue into a change when none exists
- [ ] 1.4 `workspace setup`: set stage `active`
- [ ] 1.5 `workspace archive`: resolve the same way, set stage `auto`
- [ ] 1.6 Register `workspace` in the dispatcher and `agent-help`
- [ ] 1.7 Tests with a fake Orca runner: branch match, linked-issue match, issue with no change, unrelated workspace
- [ ] 1.8 README "Working with Orca": the `orca.yaml` recipe and the one-status-writer rule
- [ ] 1.9 `site/features.json`, specsync skill
- [ ] 1.10 Narrow `in-flight-work-claims` and sync it
- [ ] 1.11 Run the recipe end to end in Orca on this repo
