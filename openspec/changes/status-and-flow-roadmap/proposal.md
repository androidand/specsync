# Roadmap: see all work, keep it concise, run one repeatable flow

## Why

Twelve planned changes belong together. Built in the wrong order, or as one large PR, they repeat the problem they solve.

This change holds the order, the dependencies and the rules for building them. It has no code.

## Goal

Specsync tells people and agents where work stands. It keeps issues and PRs short. It gives one repeatable flow, configured per repo.

It stays general: any tracker, any agent harness, any repo layout.

## The changes

| Change | Gives |
|---|---|
| `make-commands-discoverable` | one `Command` type per command; a parity test; generated docs |
| `status-pr-and-worktree-state` | `status`: PR, checks, review, local state, base and upstream movement, sync advice |
| `work-relations-and-overview` | relations graph, related work in `status`, `overview`, workspace discovery |
| `status-resume-context` | resume hint, last note, optional session provider |
| `ways-of-working-and-next` | `openspec/specsync.yml` flow and gates, `trust`, `next`, `flow` |
| `worktree-cleanup` | `cleanup`: finished work removed safely |
| `flow-verbs` | `new`, `start`, `save`, `gates`, `pr`, `update`, `park` |
| `concise-change-text` | `lint`, short PR body template, standard titles |
| `pr-size-advisory` | size and mixed-concern flags |
| `plan-changes-as-slices` | `## Slice` blocks, `slices`, `slice start` |
| `review-map-for-prs` | review checklist; proof a rewrite kept the content |
| `verify-doc-references` | stale SHA and request references |

## Order

1. Foundation: `make-commands-discoverable`. Everything else declares its commands through it.
2. Facts: `status-pr-and-worktree-state`.
3. Relations: `work-relations-and-overview`, then `status-resume-context`.
4. Config: `ways-of-working-and-next`.
5. Flow: `worktree-cleanup`, `flow-verbs`, `concise-change-text`.
6. Leaves, in any order after step 2: `pr-size-advisory`, `plan-changes-as-slices`, `review-map-for-prs`, `verify-doc-references`.

After step 2, steps 3, 4 and 6 can run in parallel. Within a change, slices go in the order given by `after:`.

## Rules

- One slice per PR. Stay within the slice budget.
- Tests first. A failing test comes before the code.
- No new dependencies. Standard library only.
- Read-only commands stay read-only. A test proves it.
- Never hand-write command docs. They are generated.
- Public repo: no home paths, private addresses, credentials or internal names.
- Reference the change's issue in every commit.
- Ask the maintainer before creating issues or releasing.

## Done when

- The parity test passes and no second list of commands exists.
- `status` and `overview` work against a real multi-repo workspace.
- A repo can set its flow, gates, text standard and sync strategy in `openspec/specsync.yml`.
- `specsync lint` passes on specsync's own proposals.
- The `topology` name no longer ships.

## Related

All twelve changes above. `START-PROMPT.md` in this folder starts the work.
