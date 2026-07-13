# Board status two-way: human board moves are signal, not noise

## Why

The first board projection treats stage → Status as one-way outbound, and its
non-clobber guard is approximate: a human dragging a card to "Done" is moved
back to "In progress" on the next sync, because specsync only whitelists
status *names* it could have written, not what it actually last wrote. That
inverts the tool's philosophy — everywhere else (task checkboxes, issue
open/closed state) external changes are reconciled inbound, not overwritten.

A board move is a statement by a human. "Done" may mean the work is done
(the local change should complete/archive) — or the next sync may legitimately
push it back because tasks reopened. Reopening is real: an issue auto-closed by
a merge may be reopened because we were not actually done. The two sides need
merge semantics, not last-writer-wins.

## What Changes

- Persist **what specsync last wrote** to the board (status option id, per
  change, in the gitignored `.specsync/` state) so "specsync-managed" means
  "unchanged since specsync set it", exactly — not name-collision guessing.
- Reconcile board Status **inbound** before projecting outbound, mirroring
  checkbox reconcile:
  - human moved to a Done-like option, local tasks incomplete → surface it
    (report + optional stage override), do not silently drag the card back
  - human moved to an active option while local stage is complete → treat as
    reopen signal, consistent with `-close-completed` reopen semantics
  - no human move since last write → project stage → Status as today
- Define precedence with the issue open/closed lifecycle: auto-close on merge
  (`-close-completed`) closes the issue; the board Done follows the close, and
  a human reopen propagates back to both stage and Status.
- Cover all cases in faked-board tests: fresh card, specsync-set then
  untouched, human-moved forward, human-moved backward, reopened issue.

### Out of scope

- Multi-board membership; custom fields beyond Status; assignee reconcile.

## Capabilities

- `board-status-reconcile`: inbound board-status merge semantics.
