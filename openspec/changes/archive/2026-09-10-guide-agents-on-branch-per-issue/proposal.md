# Guide agents toward one branch, one worktree per issue

## Why

Agents working with specsync-managed projects are often unclear on basic
git hygiene: which branch to work on, whether to reuse one across unrelated
issues, and when a worktree is warranted. specsync already has real support
for this — `pull -worktree` creates a dedicated branch and worktree per
issue — but nothing an agent actually reads mentions it.

The gap is specific: this repo's own `AGENTS.md` documents the convention in
detail, but `AGENTS.md` is a dogfooding doc for *this* repo — it is never
installed into a project that adopts specsync. `specsync install-skill` only
ever ships `SKILL.md`; `skills/specsync/docs/*.md` never leaves this repo
either. So an agent working in an arbitrary consuming project, or reading
`specsync agent-help` (the one thing guaranteed to be reachable everywhere,
since it ships with the binary), gets no guidance on this at all — and
`agent-help`'s own overview previously just said "read AGENTS.md," which is
silently false for most projects.

## What

- `specsync agent-help` (overview, both text and `-json`) now states the
  one-issue/one-branch/one-worktree convention directly, and softens the
  AGENTS.md pointer to "if the project has one."
- `specsync agent-help pull` documents the existing `-worktree`/
  `-worktree-dir` flags (previously undocumented anywhere) and adds a safety
  rule pointing at them.
- The skill (`SKILL.md`, `docs/reference.md`, `docs/workflow.md`) mentions
  the same convention and the `-worktree` flag, including the honest caveat
  that there's no equivalent automation for the spec-first path yet — branch
  and worktree by hand there.

No new command or flag: `pull -worktree` already existed and did the right
thing. This only makes it discoverable.

## Release note

`specsync agent-help` now guides agents toward one branch and worktree per
issue, and documents `pull`'s existing `-worktree`/`-worktree-dir` flags.
