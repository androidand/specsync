# Tasks — Claim work in flight

> Each phase is independently useful. Phase A alone would have caught incidents
> (1) and (3); Phase C alone would have caught (4) and (5).

## Phase A — Claims

- [ ] A.1 `specsync claim -change <id> [-worktree <path>] [-branch <b>]
      [-agent <name>] [-globs <a,b,...>] [-ttl <dur>]`. Default worktree/branch
      from the cwd's git state, agent from `SPECSYNC_AGENT` or hostname+pid.
- [ ] A.2 Store in gitignored `.specsync/claims/<change>.json`. Confirm
      `.specsync/` is already gitignored in consuming repos; the README says not
      to commit it, but verify rather than assume.
- [ ] A.3 `specsync release -change <id>`, plus TTL expiry so a crashed agent
      does not hold a claim indefinitely.
- [ ] A.4 `specsync claims [-json]` — change, agent, branch, worktree, globs,
      age, expiry.
- [ ] A.5 Overlap detection: claiming globs intersecting a live claim fails and
      names the holder. `-force` to override, because a human reassigning work
      is legitimate.

## Phase B — Worktree hygiene (dropped)

Same-worktree and volatile-path detection are dropped: per-change workspaces
(Orca, `specsync start`) prevent incidents (1) and (2). See the proposal's
"Scope narrowed by workspace orchestration".

## Phase C — Landed detection that survives squashes

- [ ] C.1 On sync, record the merge/squash commit for a change once its issue
      closes, so "landed?" is a recorded fact rather than an ancestry query.
- [ ] C.2 Warn when a change's issue is open but its content is already on the
      default branch. Real case: master held a single-parent commit titled
      "merge: integrate <branch> into master"; the PR stayed open indefinitely
      and `git merge-base --is-ancestor` reported "not merged".
- [ ] C.3 In that state, also warn that the branch may now be *behind* master —
      re-merging the stale branch would have regressed two files (569→175 and
      206→134 lines).
- [ ] C.4 Nudge on discovery: a squash whose subject begins `merge:` is the
      signature. Report it; do not try to fix history.

## Phase D — Verification provenance

- [ ] D.1 Allow a verification note to carry the commit SHA it was measured at.
- [ ] D.2 Render it stale when that SHA is no longer the branch tip. Real case:
      "8 instruction tests, 20 build-route tests pass" was true at one PR; the
      next PR broke three tests in that same service and the claim was repeated
      unchanged.
- [ ] D.3 Note in the issue body when a PR merged with `--admin` (checks
      bypassed), so a red default branch is attributable rather than mysterious.

## Phase E — Validation

- [ ] E.1 Unit tests for claim/release/TTL/overlap.
- [ ] E.2 Replay incidents (3)–(6) from the proposal as a test fixture and
      assert the tooling reports it. An incident that cannot be replayed is not
      evidence the feature works.
- [ ] E.3 Dogfood on a real two-agent session before recommending it.

## Notes

- Claims are **advisory**, never locks. Making collisions visible is the goal;
  a tool that blocks writes will be bypassed and then trusted wrongly.
- Every incident here shares one shape: **a failure that presents as success.**
  A squash labelled "merge:" looks merged. A green test claim looks current. An
  optional call that silently no-ops looks like a working control. The value of
  this change is turning that class of silence into a signal.
