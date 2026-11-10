# Scratch specs and a default store for repos that keep no `openspec/`

## Context

Builds on `smooth-first-use-in-a-new-repo` (2026-10-27), which records that
`pull` in a repo without `openspec/` fails unhelpfully and writes no `tasks.md`.
That change was not implemented, and the same failure recurred on 2026-11-09 in
the same repo (`acme/app`, issue #30):

- `specsync pull -issue 30` failed with `no openspec/ directory … (pass -openspec, -store, or declare store: …)`.
- The agent took that as "this repo has no OpenSpec setup", treated creating one
  as a repo-structure decision, searched other repos, and stopped to ask. The
  user's answer was the known routine: make a scratch `openspec/` in the
  worktree, pull, sync, discard. It took seconds once said.
- The routine is not written anywhere an agent reads; the `start` skill says it
  pulls the spec "when the repo has `openspec/`", which reads as "skip it".

Some teams' rule is that product repos carry no `openspec/`, so a per-repo
`openspec/config.yaml` is not an answer. (That repo also lists `openspec/` in
`.gitignore`; a scratch dir must still work where it is not.)

## What is already covered elsewhere

In `smooth-first-use-in-a-new-repo`, not repeated here: a helpful error for a
missing `openspec/`, a starter `tasks.md` from `pull`, warning when `openspec/`
is gitignored, and "never create `openspec/` silently". This change follows that
last rule: nothing is created without an explicit `init` or `-init`.

## Proposed changes (only what is new)

### 1. `specsync init`: a scratch spec root that stays out of git

Creates `./openspec/changes/` and appends `/openspec/` to the clone's
exclude file (`git rev-parse --git-path info/exclude`, shared by the clone's
linked worktrees; never edits a tracked `.gitignore`). Idempotent, prints what it did,
refuses where `openspec/` is already tracked. The `-init` option on `pull`
proposed in the other change calls this.

### 2. Name the command in the error

The missing-`openspec/` error (the other change's item 1) must name
`specsync init` and `-store`, and say that a repo with no specs is normal here:

```
no openspec/ directory at <dir>.
For a scratch spec in this worktree: specsync init
For a shared store: -store <id>, or set a default store
```

### 3. User-level default store

An optional `~/.config/specsync/config.yaml` (`default-store: <id>`) or
`SPECSYNC_DEFAULT_STORE` selects a store, via the existing store support
(`openspec-store-support`), when the repo has no local `openspec/` and no flag is
given. Resolution order: `-openspec` → `-store` → `config.yaml store:` →
`./openspec` → default store. This is the home for specs once a shared
"openspec repo" exists, while product repos stay free of `openspec/`.

### 4. Tell agents the routine

Update the specsync skill, `agent-help pull`/`init`, the `start` and `pr`
skills and `wt`: in a repo without `openspec/`, run `specsync init`, then
`pull`, work, `specsync -change <slug>`, and never commit the folder.

## Out of scope

Creating or choosing the shared store repo.
