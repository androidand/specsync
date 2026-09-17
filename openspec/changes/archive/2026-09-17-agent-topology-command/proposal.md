# Show where each change's issue, branch, and worktree live

## Context

An agent working one part of a multi-repo change cannot answer basic questions about
the rest of it: which other repos are involved, which issues cover them, which branches
and worktrees hold that work, and whether anyone has started.

Every fact needed to answer this is already inside specsync. It is just not emitted as
one table:

- `ReadCoordination` (`coordination.go`) already resolves the root store and its
  referenced sibling stores with local paths, via `openspec context --json`.
- `.specsync/refs.json` per change already holds the tracker provider, issue id and URL.
- `specsync epic` already wires cross-repo parents and children.
- `specsync link` already cross-links sibling changes.
- The branch/worktree convention (`feat/<n>-<slug>`, `../worktrees/<slug>`) is already
  established and automated by `pull -worktree`.

Consumers exist and are blocked on this. `opencode-skein`'s `agent-presence` model knows
which session is running in which directory, but a directory is not a unit of work — it
cannot say *which change* a peer is working on without this join. The join key is the
change slug, reached through the worktree path.

## Proposed Changes

### New Subcommand: `specsync topology`

Emits the join table for active changes. Read-only; mutates nothing.

```bash
specsync topology                      # human-readable table
specsync topology -json                # machine-readable
specsync topology -change <slug>       # scope to one change
specsync topology -all                 # include archived
```

JSON shape, one entry per change:

```json
{
  "changes": [
    {
      "slug": "add-field-forms",
      "title": "Auto-render field forms",
      "stage": "active",
      "store": "/Users/andreas/exopen/portal",
      "repo": "ExopenGitHub/portal",
      "issue": {"provider": "github:ExopenGitHub/portal", "id": "4231",
                "url": "https://github.com/ExopenGitHub/portal/issues/4231",
                "state": "open"},
      "branch": "feat/4231-add-field-forms",
      "worktree": "/Users/andreas/exopen/portal-worktrees/add-field-forms",
      "epic": "https://github.com/ExopenGitHub/planning/issues/88",
      "linked": ["add-field-forms-api"]
    }
  ],
  "status": ["gh unavailable: issue state omitted"]
}
```

### Where each field comes from

- `slug`, `title`, `stage` — the OpenSpec change, as `specsync changes` already reads it.
- `store`, `repo` — `ReadCoordination` plus git remote detection.
- `issue` — `.specsync/refs.json`. Issue **state** needs `gh` and is therefore optional.
- `branch`, `worktree` — `git worktree list --porcelain` in each coordinated store,
  matched to the change by branch name (`feat/<n>-<slug>`) first, then by directory
  basename. An unmatched change reports `worktree: null`, not an error.
- `epic`, `linked` — existing epic wiring and `specsync link` edges.

### Degradation

Follows the discipline `coordination.go` already sets: every source may be missing, and a
missing source narrows the answer rather than failing it. No `openspec` binary, no `gh`
auth, an uncloned sibling store, or a store with no worktrees each drop their fields and
append a line to `status[]`. `topology` exits 0 whenever it produced any rows at all.

This matters more here than elsewhere: the consumer is an agent deciding whether to start
work. A command that fails closed teaches agents to skip the check.

## Capabilities

### New Capabilities

- `agent-topology`: a single read-only projection joining change, tracker issue, git
  branch and git worktree across coordinated stores.

## Non-Goals

- **No presence, liveness, or process awareness.** Which agent is *running* where is the
  runtime's business, not specsync's. This command emits the map; something else reads
  the radio. Keeping that line is what lets specsync stay agent-neutral.
- No claiming, locking or assignment. Reporting that a worktree exists is not reserving it.
- No new state. Every field is derived from data specsync or git already stores; nothing
  here introduces a file that can go stale independently.
- No worktree creation. `pull -worktree` already owns that.
- No cross-machine view. Local stores only.

## Impact

- New: `topology.go`, `cmd/specsync/` wiring, `agent-help topology` entry.
- Changed: `install-skill`'s embedded `SKILL.md` gains a short "check before you start"
  step pointing at `specsync topology -json`.
- Reuses `ReadCoordination`, the refs cache and the changes lister; no new dependencies.
