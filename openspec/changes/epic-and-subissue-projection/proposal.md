# Epic & sub-issue projection

## Why

Larger efforts span multiple repos and phases. A common pattern is one "epic"
issue that coordinates several sub-issues, where each sub-issue is the unit that
gets a branch, a worktree, and — once implementation starts — a focused spec.
specsync should understand this hierarchy so an epic stays a coordination shell
while each sub-issue maps one-to-one to an OpenSpec change.

GitHub now models this natively: **sub-issues** (parent↔child) went GA in 2025,
support cross-repo/cross-org via the GraphQL `subIssueUrl`, and are read/written
through `gh api graphql` (`subIssues`, `parent`, `subIssuesSummary`;
`addSubIssue`/`removeSubIssue`/`reprioritizeSubIssue`). So the hierarchy should be
projected onto real sub-issues — not a markdown checklist that only renders.

## What Changes

- Recognize an **epic** by convention (a `type:epic` label/marker), not by having
  a spec of its own — an epic is a coordination issue.
- Add a typed **`parent`** edge to a change's `links.md` (the committed relationship
  layer; never `refs.json`, which stays identity-only). A child change with a
  `## Parent` entry is projected as a **sub-issue** of that parent via the GitHub
  sub-issues API, cross-repo-safe through `subIssueUrl`.
- **Reconcile the parent edge both ways** against a last-synced **baseline** (a
  gitignored snapshot in `.specsync/`, the merge base). Because an edge is binary
  (present/absent), the 3-way reconcile has no ambiguous conflict: an edge added in
  `links.md` is pushed to GitHub; a parent attached on GitHub by a human is
  **pulled into `links.md`**; an edge removed on either side is removed from the
  other. `links.md` and the tracker converge — neither goes stale, and a human's
  UI edit is honored rather than discarded.
- **Roll up** the epic body from its live `subIssuesSummary` (count + completion),
  while each sub-issue's body stays driven by its change's proposal + tasks.

### Out of scope / explicitly deferred
- `blocked-by` / `blocks` dependencies (→ `issue-dependency-sync`)
- Deriving relationships from OpenSpec `references:` (→ `openspec-references-coordination`)
- Reordering sub-issues (`reprioritizeSubIssue`) — add when a real need appears
- Non-GitHub providers — sub-issue projection is a GitHub capability; others gain
  an equivalent when they exist (`pluggable-providers`)

## Capabilities

### New Capabilities
- `sub-issue-projection` — project a child change's `## Parent` edge onto a GitHub
  sub-issue (cross-repo via `subIssueUrl`), delta-reconciled against `links.md`,
  with epic-body roll-up from `subIssuesSummary`.

## Impact
- New code: typed-edge parsing in `links.md` (a `## Parent` section beside the
  existing `## Related`), and a sub-issue reconcile that shells `gh api graphql`.
- Reuses the asserted-graph line (`work-graph`) and the per-layer reconcile model
  (`two-way-reconcile`); stdlib-only; shells out. Builds on the typed-`links.md`
  groundwork in `link-by-issue-reference`.
