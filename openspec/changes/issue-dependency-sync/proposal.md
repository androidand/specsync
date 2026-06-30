# Sync issue dependencies (blocked-by / blocks)

## Why

Work that spans a backend and a frontend almost always has a *direction*: the
frontend change can't ship until the backend change lands. Today specsync can
record that two specs are "related" (`## Related`), but "related" is symmetric and
loses the dependency direction — the exact thing a planner and an agent need to
sequence the work.

GitHub now models this as a first-class relationship: **issue dependencies**
(`blockedBy` / `blocking`) went GA in 2025, with real mutations (`addBlockedBy`,
`removeBlockedBy`) and a summary field (`issueDependenciesSummary`), read/written
via `gh api graphql` (there is no native `gh issue` flag yet). So a directed
local edge should project onto a real dependency, not be flattened into "related".

## What Changes

- Add directed typed edges to `links.md`: **`## Blocked by`** and **`## Blocks`**
  (the inverse). Entries take the usual forms (`#N` / `owner/repo#N` / URL), so a
  backend issue in another repo can block a frontend change.
- On sync, project each `## Blocked by` entry onto a GitHub **issue dependency**
  via `addBlockedBy`, cross-repo by node id. `## Blocks` is the same edge seen from
  the other end (write it as the blocker's `blockedBy`).
- **Reconcile dependencies both ways** against a last-synced **baseline** (a
  gitignored snapshot in `.specsync/`, the merge base). Each edge is binary, so the
  3-way reconcile is unambiguous: an edge added in `links.md` is pushed
  (`addBlockedBy`); a dependency added on GitHub by a human is **pulled into
  `links.md`**; an edge removed on either side is removed from the other
  (`removeBlockedBy` for the GitHub side). Dependencies converge in both
  directions — none go stale, and UI-added dependencies are honored, not discarded.

### Out of scope / explicitly deferred
- Parent/sub-issue hierarchy (→ `epic-and-subissue-projection`)
- `duplicateOf` and other relationship types — add when a real need appears
- Cycle detection across dependencies — GitHub rejects cycles itself; specsync
  surfaces the error rather than pre-validating
- Non-GitHub providers — dependency sync is a GitHub capability; others gain an
  equivalent when they exist (`pluggable-providers`)

## Capabilities

### New Capabilities
- `issue-dependency-sync` — project directed `## Blocked by` / `## Blocks` edges
  from `links.md` onto GitHub issue dependencies, delta-reconciled, cross-repo.

## Impact
- New code: parse `## Blocked by` / `## Blocks` in `links.md`; a dependency
  reconcile that shells `gh api graphql` (`issueDependenciesSummary`, `blockedBy`;
  `addBlockedBy` / `removeBlockedBy`).
- Same asserted-graph and per-layer-reconcile model as the sibling changes;
  stdlib-only; shells out. Composes with `epic-and-subissue-projection` (both are
  typed `links.md` edges projected onto native GitHub relationships).
