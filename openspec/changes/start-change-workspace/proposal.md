# Start a change in its own workspace

## Why

Beginning work on a change takes several steps an agent has to get right from
memory: find the issue number, name the branch `feat/<issue>-<slug>`, base it
on the current remote default branch, create a worktree, and mark the change
active. Weaker models skip the worktree and commit on the default branch.

Workspace orchestrators such as [Orca](https://www.onorca.dev) already do the
worktree part well and link each workspace to its tracker issue
(`orca worktree create --issue <n>`). The issue Orca hands the agent is the
spec as specsync renders it, so the two agree on what the work is. What is
missing is one command that goes from a change to a running workspace.

Depends on `infer-change-from-branch`: once the workspace exists, every later
command inside it finds the change from the branch.

## What Changes

- New `specsync start <slug>`:
  - Refuses a change with no linked issue and prints the sync command to run,
    because the branch name and changelog binding need the issue.
  - Branch `feat/<issue>-<slug>`, from the fetched remote default branch.
  - `-launcher auto|orca|git` (default `auto`). With Orca it runs
    `orca worktree create --name <slug> --branch <branch> --base-branch
    <default> --issue <n>`, passing `-agent` and `-prompt` through. Without it
    it creates the worktree with git, at the path `pull -worktree` uses.
  - The Orca executable is resolved as Orca's own guide prescribes:
    `$ORCA_CLI_COMMAND`, then `orca-ide` on Linux (bare `orca` there is the
    GNOME screen reader), then `orca`.
  - Reuses an existing worktree for the change instead of creating another.
  - Sets the stage to `active`; syncs the tracker only with `-sync`.
  - `-dry-run` prints the branch, path and launcher command.
- Branch and worktree naming move into one helper shared with `pull
  -worktree`. That fixes two `pull -worktree` quirks: the worktree was named
  `<owner>/<repo>-<issue>` (a nested folder) instead of `<repo>-<issue>`, and
  `../worktrees` was taken from the current directory rather than the main
  checkout.
- Orca is called with `--no-parent`: a change is independent work, not a child
  of whatever workspace `start` runs in. A branch that already has a worktree is
  detected before Orca is called, because Orca suffixes a taken branch name
  (`-2`) instead of reusing it.
- `start` warns when the change's plan is not on the remote default branch,
  since a worktree cut from there will not contain it.

specsync stays invoke-and-exit: it starts the workspace and never supervises
the agent.

## Capabilities

### New Capabilities

- `workspace-start`: start a synced change in its own worktree, through an
  orchestrator when one is installed.

## Impact

- New `start` subcommand in `cmd/specsync/`, registered in `agent-help`.
- Naming extracted from `runPullWithWorktree` (`cmd/specsync/main.go`).
- README, `site/features.json`, the specsync skill.
- Orca is optional and only shelled out to; no new dependency.

## Release note

`specsync start <slug>` opens a synced change in its own worktree, through
Orca when it is installed.
