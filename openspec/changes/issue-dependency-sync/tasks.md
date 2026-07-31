# Tasks: sync issue dependencies

## Typed links (`links.md`, core)
- [x] Parse `## Blocked by` and `## Blocks` sections (entries: `#N` / `owner/repo#N` / URL)
- [x] Treat them as directed edges, distinct from the symmetric `## Related`

## Dependency projection (GitHub, `gh api graphql`)
- [x] Read current dependencies (`issueDependenciesSummary`, `blockedBy`, `blocking`)
- [x] Resolve node ids for cross-repo references; `addBlockedBy` for `## Blocked by`
- [x] `## Blocks` projects as the named issue's `blockedBy` (the inverse edge)
- [x] Maintain a gitignored `.specsync/` baseline of the last-synced dependency set (the merge base)
- [x] Reconcile both ways against the baseline: push local adds, pull GitHub adds into `links.md`, `removeBlockedBy` for local removals, remove from `links.md` for GitHub removals; update the baseline to the converged set
- [x] Surface GitHub's error on an invalid/cyclic dependency rather than pre-validating

## Boundaries & tests
- [x] Stdlib-only; shell out to `gh api graphql`; `boundary_test.go` green
- [x] Fake-runner tests: add blocked-by, inverse `## Blocks`, cross-repo, remove-on-removal, unmanaged-edge gap, cycle-error surfaced
- [x] Update the specsync skill with the `## Blocked by` / `## Blocks` syntax
