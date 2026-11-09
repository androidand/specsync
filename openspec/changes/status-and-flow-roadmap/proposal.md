# Roadmap: see all work, keep it concise, run one repeatable flow

## Why

Twelve planned changes belong together, plus a few independent ones. Built in the wrong order, or as one large PR, they repeat the problem they solve.

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

Independent changes, not part of the order below. They touch no shared files with the twelve beyond
docs, so they can land at any time:

| Change | Gives |
|---|---|
| `smooth-first-use-in-a-new-repo` | `pull` works without `openspec/`; starter `tasks.md`; scoped `validate`; `pr-body` link notice |
| `scratch-spec-and-default-store` | `init`, a scratch `openspec/` kept out of git; default store |
| `archive-close-reason` | `archive` records why an issue was closed |
| `changes-show-issue-state` | `changes -issues`: issue state and drift |

## Order

1. Foundation: `make-commands-discoverable`. Everything else declares its commands through it.
2. Facts: `status-pr-and-worktree-state`.
3. Relations: `work-relations-and-overview`, then `status-resume-context`.
4. Config: `ways-of-working-and-next`. Slice 0 (the config parser) comes first.
5. Text and leaves: `concise-change-text` (needs step 4 for `text:`), and in any order after step 2 `pr-size-advisory`, `plan-changes-as-slices`, `review-map-for-prs`, `verify-doc-references`.
6. Flow: `worktree-cleanup` (needs step 4 for `trust` and `cleanup:`), then `flow-verbs`. Its slices 1 to 3 and 5 need only step 4. Its slice 4 (`pr prep`, `pr open`) needs `concise-change-text`, `pr-size-advisory` and `review-map-for-prs` from step 5.

After step 2, steps 3, 4 and 5 can run in parallel, except where a step names its own need. Within a change, slices go in the order given by `after:`.

## Rules

- One slice per PR. Stay within the slice budget.
- Tests first. A failing test comes before the code.
- No new dependencies. Standard library only.
- Read-only commands stay read-only: a test proves the worktree, index, refs and config are unchanged. Unreferenced git objects may be added (see `git merge-tree`).
- Never hand-write command docs. They are generated.
- A command that shipped is renamed with a deprecated alias for one release, never removed outright.
- Public repo: no home paths, private addresses, credentials or internal names.
- Reference the change's issue in every commit.
- Ask the maintainer before creating issues or releasing.

## Done when

- The parity test passes and no second list of commands exists.
- `status` and `overview` work against a real multi-repo workspace.
- A repo can set its flow, gates, text standard and sync strategy in `openspec/specsync.yml`.
- `specsync lint` passes on specsync's own proposals.
- `topology` is a deprecated alias of `status` for one release, then removed.

## Related

All changes above. `START-PROMPT.md` in this folder starts the work.
