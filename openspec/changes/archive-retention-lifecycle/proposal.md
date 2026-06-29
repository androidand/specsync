# Archive & retention lifecycle

OpenSpec has only two lifecycle states — active or archived — derived from folder
location. That leaves teams with a bad binary: keep every archived change in git
forever (structured landfill: noisy diffs, stale claims, low signal) or delete
them and lose the intent ("why did we design it this way?").

specsync already dissolves this binary but doesn't yet act on it. The projected
issue carries the change marker, the proposal, and the final task state. That
issue *is* a durable, searchable, owned, zero-repo-noise record of intent. So the
honest retention model is not "keep concise specs in git forever" — it's
**compact finished work into the tracker, keep the tree lean.**

## Solution

**`specsync archive <slug>`** — a real lifecycle step, not just a folder move:

1. **Reconcile** — final push so the issue reflects final scope and task state.
   Report any unchecked tasks; refuse to archive unless `-force`.
2. **Close + label** — close the issue and apply `spec:archived` so the tracker
   shows the change as settled, queryable history.
3. **Retain** — apply a retention policy to the local folder:
   - `move` (default): relocate to `openspec/changes/archive/<slug>/` (OpenSpec
     native), keeping the proposal in git.
   - `prune`: remove the local folder entirely — the closed issue holds the
     intent. For teams that treat the tracker as the archive of record.

**Curation gate.** A change is *significant* (worth a full kept archive) or
*trivial* (issue-as-archive is enough). Drive retention from a per-change signal:
a `significant` marker file, or a heuristic (has `design.md`, touches > N tasks).
Trivial changes default to `prune`; significant ones default to `move` — the
anti-bloat rule made mechanical: **no encyclopedic archive for trivial work.**

## Configuration

Resolve policy in order: `-retain move|prune` flag → `.specsync/config` in the
repo → significance heuristic default. Keep it a few plain keys; no new format.

## Provider agnosticism

Close + label go through the `WorkProvider` interface (GitHub passes `--state
closed` / label ops to `gh`). Retention is purely local file movement. Future
providers need nothing new beyond a `Close` they already implement.

## Non-goals

- **Bloat lint** (proposal-to-diff ratio, design.md on trivial changes) — a
  separate change; archive only consumes the significance signal, it doesn't
  compute style violations.
- **Drift / dead-spec detection** (code changed but spec didn't) — separate
  change.
- Writing compacted memory into Beads — see `beads-memory-bridge`, which hooks
  this command's archive event.
