# Design: never-fail-a-sync-silently

## Taxonomy of a change's outcome

| Outcome | Meaning | Exit code |
| --- | --- | --- |
| created / updated | the tracker now matches the spec | 0 |
| failed | the change was attempted and could not be synced (tracker error, refused size, fork refusal) | 1 |
| no-op | nothing to write | 0 |

A failed change never stops the others: the run continues, then prints a trailing
block naming each failed change and exits 1. The summary line carries the count
(`27 created, 16 updated, 1 failed`).

A change skipped because it has no `proposal.md` is not an attempted sync. It is not
counted as failed; reporting such skips is tracked separately and stays open here.

## Oversized body

Decision: a hard refusal, made in the GitHub provider before any network call, after
`design.md` has had its chance to overflow to a comment. The error names the change,
the length, the limit, the largest part of the body and the remedy. The check also
runs under `-dry-run`, because a dry run uses the same `Push`.

The check lives at sync time only. Catching it at authoring time would put specsync
knowledge into `openspec validate`, which specsync does not own.
