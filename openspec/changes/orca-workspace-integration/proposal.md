# Start each change in its own workspace

## Why

Workspace orchestrators such as [Orca](https://www.onorca.dev) now handle the
execution side of agent work: one worktree per task, one agent per worktree,
diffs, PRs, and a worktree record that links to its tracker issue
(`orca worktree create --issue <n>`). They start work *from issues* but do not
know where those issues came from or whether they match a spec.

That is the gap specsync already fills (spec → issue → changelog), so the two
compose:

    OpenSpec change ─specsync─▶ issue ─orchestrator─▶ worktree + agent + PR ─specsync─▶ changelog

Today the hand-off in the middle is manual and convention-based. An agent has
to know this repo's branch shape (`feat/<issue>-<slug>`), find the issue
number, and remember to create a worktree at all. Weaker models skip the
worktree and commit on the default branch; nothing in specsync stops them, and
the written rules in `AGENTS.md` are exactly what they ignore.

Some of specsync's open scope also duplicates what an orchestrator now
guarantees by construction (two agents in one worktree, worktrees in volatile
paths). That scope should narrow rather than compete.

## What Changes

- **`specsync start <slug>`**: one command to begin work on a synced change.
  - Resolves the change's linked issue; refuses (with the sync command to run)
    when the change has no issue yet, because the branch name and the
    changelog binding both need it.
  - Derives the branch `feat/<issue>-<slug>` and the worktree path with the
    same helper `pull -worktree` uses, so the two can never disagree.
  - Branches from the fetched remote default branch, never from whatever the
    current checkout happens to be on.
  - Launcher `-launcher auto|orca|git` (default `auto`): with Orca available it
    runs `orca worktree create --name <slug> --branch <branch> --base-branch
    <default> --issue <n> [--agent <id>] [--prompt <text>]`; otherwise it falls
    back to `git worktree add`. The Orca executable is resolved the way Orca's
    own guide prescribes (`$ORCA_CLI_COMMAND`, then `orca-ide` on Linux, where
    bare `orca` is the GNOME screen reader, then `orca`).
  - Idempotent: if a worktree already exists for the change (matched as
    `topology` does), it reports it and creates nothing.
  - Sets the change's stage to `active` (same effect as `set-stage <slug>
    active`); it does not sync the tracker unless `-sync` is passed.
  - `-dry-run` prints the resolved branch, path and launcher command.
  - specsync stays invoke-and-exit: it starts nothing long-lived and never
    supervises the agent.
- **Infer the change from the current branch.** Commands that take a change
  (`set-stage`, `archive`, `note`, the default sync) accept no change argument
  when the current branch matches `feat/<issue>-<slug>`. On the default branch
  or a non-matching branch they keep failing with "no change given", which is
  the point: an agent on `main` cannot silently act on a change.
- **Orca repository hook recipe.** Document an `orca.yaml` that keeps the
  change stage in step with the workspace lifecycle, using branch inference
  (Orca's hooks receive `ORCA_WORKTREE_PATH`, `ORCA_WORKTREE_ID` and
  `ORCA_ROOT_PATH`, not the linked issue, so the branch is the only reliable
  key):

  ```yaml
  scripts:
    setup: |
      specsync set-stage active
    archive: |
      specsync set-stage auto
  ```

- **Narrow `in-flight-work-claims`.** Rescope it to what an orchestrator does
  not cover: detecting whether a squash-merged change has landed (its
  incidents 4 and 5) and overlapping files *across* worktrees (incident 3).
  Drop same-worktree collision and volatile-path detection (incidents 1 and
  2), which per-worktree orchestration prevents.
- **Positioning.** README "Why" and the site state the layering plainly:
  specsync owns spec ↔ tracker ↔ changelog; worktrees and agents belong to the
  orchestrator.

### Out of scope

- Supervising, messaging or monitoring agents (orchestrator's job).
- Reading Orca's state (`orca worktree list --json`) into `topology`. Useful
  later; `topology` already finds the worktree by branch.
- A guard that blocks commits on the default branch. Worth its own change.
- `work-graph` (deferred): an orchestrator's task DAGs cover it; revisit or
  drop separately.

## Capabilities

### New Capabilities

- `workspace-start`: start work on a synced change in its own worktree,
  through an orchestrator when present, and resolve the change from the
  current branch.

## Impact

- New `start` subcommand in `cmd/specsync/`, registered in `agent-help`.
- Branch/worktree naming extracted from `runPullWithWorktree`
  (`cmd/specsync/main.go`) into one shared helper; `worktreeBranchPattern`
  (`topology.go`) becomes its inverse.
- Branch-based change inference shared by the commands listed above.
- Docs: README, `site/features.json`, the specsync skill.
- No new runtime dependency: Orca is optional and only shelled out to.

## Release note

`specsync start <slug>` opens a synced change in its own worktree, through
Orca when it is installed, and commands can now find the change from the
branch you are on.
