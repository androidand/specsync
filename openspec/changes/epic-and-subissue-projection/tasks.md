# Tasks: Epic & sub-issue projection

## Typed links (`links.md`, core)
- [ ] Parse a `## Parent` section in `links.md` (one entry: `#N` / `owner/repo#N` / URL), beside the existing `## Related`
- [ ] Keep `## Related` behavior unchanged; `refs.json` stays identity-only

## Sub-issue projection (GitHub, `gh api graphql`)
- [ ] Read the issue's current parent/sub-issues (`parent`, `subIssues`, `subIssuesSummary`)
- [ ] Attach a child via `addSubIssue` using `subIssueUrl` (cross-repo/cross-org safe); resolve node ids as needed
- [ ] Maintain a gitignored `.specsync/` baseline of the last-synced parent edge (the merge base)
- [ ] Reconcile both ways against the baseline: push local add, pull GitHub add into `links.md`, remove on the opposite side for a removal recorded in the baseline; `removeSubIssue` for a local removal; update the baseline to the converged set

## Epic handling
- [ ] Detect `type:epic`; do not require or create a change/spec for the epic
- [ ] Roll up the epic body from `subIssuesSummary` (total/completed); never overwrite a child's body

## Boundaries & tests
- [ ] Stdlib-only; shell out to `gh api graphql`; `boundary_test.go` green
- [ ] Fake-runner tests: attach, detach-on-removal, cross-repo via URL, unmanaged-edge gap, epic roll-up
- [ ] Update the specsync skill with the `## Parent` syntax and the epic convention
