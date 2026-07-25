# Fix task dogfooding: audit tasks and reconcile with code

## Why

specsync failed its own dogfooding: 30+ active changes had unchecked tasks while
code was already implemented. The changelog was the public proof that specsync
works — but the tasks were the internal proof, and they were silently wrong.

The root problem: tasks.md drifted from reality. Tasks were checked off during
planning, then implementation happened without updating the checklist. No audit
surface existed to catch this drift.

This change adds the `specsync audit-tasks` command to detect the drift, runs it
in CI to enforce hygiene, and reconciles the existing drift by auditing every
active change against its actual implementation state.

## Release note

Added `specsync audit-tasks` to detect unchecked tasks where code exists,
enforced via CI. Reconciled 47 changes — checked off done tasks, spun off
follow-ups, fixed design docs that didn't match code.
