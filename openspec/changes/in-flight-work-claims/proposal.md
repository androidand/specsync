# Claim work in flight so concurrent agents don't collide

## Why

`openspec-references-coordination` solves *cross-repo* awareness: two repos, two
worktrees, "the other one exists and here is where". This proposal is the
neighbouring problem it does not cover: **one repo, several agents, overlapping
files, at the same time**.

Every item below is a real incident from a single day on `androidand/brick-now`,
with three agents (two coding, one recovering PRs) working the same repo.

**1. Two agents wrote to the same worktree.** Agent A was mid-task in
`brick-now-worktrees/step-source`; Agent B committed and pushed to the same
branch, in the same directory. Nothing detected it. The only reason no work was
lost is that A happened to have a clean tree at that moment.

**2. Uncommitted work was destroyed by a `/tmp` cleanup.** A worktree lived in
`/tmp/bricknow-step-source`; macOS cleaned it between sessions. A shared helper
module, 17 converted call sites and a test file were gone. The branch survived —
only the uncommitted delta died. Nothing warned that a worktree holding
uncommitted work sat in a volatile path.

**3. Two agents independently implemented the same fix.** Agent A had already
landed a plan→parts projection with a mutation-tested guard and corpus evidence.
Agent B, from the OpenSpec change alone, announced it was about to write "a
shared pure adapter over the canonical plan" — the same function. Both changes
were legitimately in scope for their respective changes; neither could see the
other's *in-progress* files. Discovering this needed a human relaying chat logs.

**4. "Merge" commits that were squashes left PRs open forever.** Master contained
`8873c2a99` titled *"merge: integrate <branch> into master"* with **one parent**.
The content was in master; none of the branch's commits were ancestors. So
`git merge-base --is-ancestor` said "not merged", GitHub kept the PR open, and an
agent nearly re-merged a branch that had since gone *stale* — merging it would
have regressed two files (569→175 and 206→134 lines).

**5. A branch was behind master in content while looking mergeable.** Same root
cause as (4): ancestry is not a reliable "is this landed?" signal once squashes
enter the history, and nothing recorded which change a squash represented.

**6. `--admin` merges bypassed CI and left master red.** Two PRs were merged with
`gh pr merge --squash --admin`. The second broke three tests in the service the
first had added. The failure was only found because another agent happened to run
the backend suite. A "verified green" claim from an earlier PR was carried
forward past the change that invalidated it.

The common shape: **specsync knows what work exists, but not what is being worked
on right now, by whom, or where on disk.** A change's issue says "active" from the
moment it is proposed until it is archived — which is precisely the window in
which collisions happen.

## What Changes

- **`specsync claim`** — record that an agent is actively working a change:
  worktree path, branch, agent identifier, and the file globs it expects to
  touch. Stored in a gitignored `.specsync/claims/` snapshot *and* projected onto
  the issue (a `claimed-by` marker), so it is visible without repo access.
  `specsync release` clears it; claims carry a TTL so a crashed agent does not
  hold a lock forever.

- **`specsync claims`** — list active claims with their paths, branches and
  globs. This is the "who is in this file right now" query that (1) and (3)
  needed. Machine-readable via `-json` for planners.

- **Overlap detection on claim.** Claiming globs that intersect another live
  claim's globs fails loudly, naming the other change, agent and worktree. That
  is (3) caught before duplicate work, not after.

- **Volatile-worktree warning.** Claiming a worktree under a
  system-temp path warns, and `specsync claims` flags any claim whose worktree
  holds uncommitted changes. That is (2): the danger is uncommitted work in a
  path the OS may reclaim.

- **Landed-detection that survives squashes.** Record the merge/squash commit for
  a change so "is this landed?" is answered from recorded fact, not ancestry.
  Surface a warning when a change's issue is open but its content is present on
  the default branch — the (4)/(5) signature.

- **Verification provenance.** Let a sync attach the commit SHA a verification
  claim was measured at. A "tests green" note pinned to a SHA that is no longer
  the branch tip is shown as stale rather than as current — the (6) failure.

## What This Does Not Do

- It is **not a lock**. Claims are advisory and observable; they do not prevent
  writes. The goal is for a collision to be *visible*, not impossible.
- It does not manage worktrees or branches. Creating, moving and removing them
  stays with git and the agent harness.
- It does not replace CI. Verification provenance records *when* a claim was
  measured; it does not run or gate anything.

## Open Questions

- Should claims live only in the gitignored snapshot, or also be committed? A
  committed claim is visible to agents that never talk to GitHub, but it churns
  the tree and will conflict — which is itself the problem being solved.
- Is per-file-glob overlap the right granularity, or should it be per-change with
  human-declared "surfaces" (backend / frontend / ingest)? Globs are precise but
  need an agent to predict its own blast radius up front, and in incident (3)
  neither agent would have predicted the shared module.
- TTL default, and whether release-on-merge should be automatic.

## Related

- `openspec-references-coordination` — cross-repo awareness; this is the
  same-repo, concurrent-agent case.
- `work-graph` (deferred) — would subsume landed-detection if it ever ships,
  since commits/PRs become first-class nodes there.
- `issue-dependency-sync` — dependency *direction*; orthogonal to *concurrency*.
