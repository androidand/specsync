# Tasks: Epic & sub-issue projection

## Typed links (`links.md`, core)
- [ ] 1.1 Parse a `## Parent` section in `links.md` (one entry: `#N` / `owner/repo#N` / URL), beside the existing `## Related`
- [ ] 1.2 Keep `## Related` behavior unchanged; `refs.json` stays identity-only

## Sub-issue projection (GitHub, `gh api graphql`)
- [ ] 2.1 Read the issue's current parent/sub-issues (`parent`, `subIssues`, `subIssuesSummary`)
- [ ] 2.2 Attach a child via `addSubIssue` using `subIssueUrl` (cross-repo/cross-org safe); resolve node ids as needed
- [ ] 2.3 Maintain a gitignored `.specsync/` baseline of the last-synced parent edge (the merge base)
- [ ] 2.4 Reconcile both ways against the baseline: push local add, pull GitHub add into `links.md`, remove on the opposite side for a removal recorded in the baseline; `removeSubIssue` for a local removal; update the baseline to the converged set

## Epic handling
- [ ] 3.1 Detect `type:epic`; do not require or create a change/spec for the epic
- [ ] 3.2 Roll up the epic body from `subIssuesSummary` (total/completed); never overwrite a child's body

## Boundaries & tests
- [ ] 4.1 Stdlib-only; shell out to `gh api graphql`; `boundary_test.go` green
- [ ] 4.2 Fake-runner tests: attach, detach-on-removal, cross-repo via URL, unmanaged-edge gap, epic roll-up
- [ ] 4.3 Update the specsync skill with the `## Parent` syntax and the epic convention
