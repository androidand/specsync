# Tasks: sync issue dependencies

## Typed links (`links.md`, core)
- [ ] Parse `## Blocked by` and `## Blocks` sections (entries: `#N` / `owner/repo#N` / URL)
- [ ] Treat them as directed edges, distinct from the symmetric `## Related`

## Dependency projection (GitHub, `gh api graphql`)
- [ ] Read current dependencies (`issueDependenciesSummary`, `blockedBy`, `blocking`)
- [ ] Resolve node ids for cross-repo references; `addBlockedBy` for `## Blocked by`
- [ ] `## Blocks` projects as the named issue's `blockedBy` (the inverse edge)
- [ ] Maintain a gitignored `.specsync/` baseline of the last-synced dependency set (the merge base)
- [ ] Reconcile both ways against the baseline: push local adds, pull GitHub adds into `links.md`, `removeBlockedBy` for local removals, remove from `links.md` for GitHub removals; update the baseline to the converged set
- [ ] Surface GitHub's error on an invalid/cyclic dependency rather than pre-validating

## Boundaries & tests
- [ ] Stdlib-only; shell out to `gh api graphql`; `boundary_test.go` green
- [ ] Fake-runner tests: add blocked-by, inverse `## Blocks`, cross-repo, remove-on-removal, unmanaged-edge gap, cycle-error surfaced
- [ ] Update the specsync skill with the `## Blocked by` / `## Blocks` syntax
