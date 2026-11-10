# Show each change's issue and its state, and flag drift

## Why

`specsync changes` lists stage and task progress but not the issue behind each change, or whether that
issue is still open. When issues are closed outside specsync (a clean-up, a merged PR, a duplicate), the local
change stays in `openspec/changes/` with no signal. In one repo, a clean-up that closed nine issues left
nine changes behind. The reverse also happens: a change with every task checked, whose issue is still open.
`audit` does not help: it only compares archived changes with PRs.

Task counts alone are not proof of anything. One change stood at 0 of 11 tasks while its work was already in
the code. So this change reports mismatches between two recorded states, and does not judge whether work is done.

## What

- `changes` gains `-issues`. It reads each change's issue ref from the ref cache and asks the provider for
  the issue states in one batched call. Without `-issues`, no network call is made and output is unchanged.
- With `-issues`, the table adds ISSUE (`#N`) and ISSUE STATE (`open` / `closed (completed|not planned|duplicate)`),
  and `-json` adds `issue`, `issueState` and `issueStateReason`.
- A DRIFT column names two mismatches, and nothing else:
  - `closed-unarchived`: the issue is closed and the change is not archived.
  - `complete-open`: the change has at least one task, every task is checked and the issue is open.
- `-drift` lists only changes with drift and exits non-zero when there is any, for a clean-up script run where the changes were synced.
- The ref cache is local to the machine and gitignored. A change with no cached ref cannot be checked, so the output ends with a count of unchecked changes. A fresh CI checkout has no cache and checks nothing; this change does not claim to serve CI.

## Out of scope

- No automatic archive or close. Each drift item is resolved by `archive` (see `archive-close-reason`) or by hand.
- No code inspection and no claim that unchecked tasks are done.
- Providers with no notion of closed state report `unknown` and never drift.

## Release note

`specsync changes -issues` shows each change's issue and state and flags changes whose issue was closed or whose tasks are all checked while the issue is open.
