# Start work through Orca

## Why

`specsync start` creates a change's worktree the same way every time. Teams
that run their agents in [Orca](https://www.onorca.dev) want that worktree to
be an Orca workspace, linked to the change's issue, so Orca's agent gets the
spec as its brief and the workspace shows up in Orca's sidebar. Today they
have to create it twice: once with `start`, once in Orca.

This replaces the separate `start` command planned in #201: `main` already
has `start` (#226), so Orca becomes a way for it to create the worktree, not a
second command.

## What Changes

- `worktrees.launcher` in `openspec/specsync.yml`: `git` (default, today's
  behaviour), `orca`, or `auto` (Orca when it is installed, git otherwise).
  `start -launcher` overrides it for one run.
- With Orca, `start` runs `orca worktree create --name <slug> --branch
  <branch> --base-branch <base> --no-parent`, plus `--issue <n>` when the
  change has an issue and `-agent` / `-prompt` when given. Everything else in
  `start` is unchanged: reusing an existing worktree, the live-session check,
  trust, the branch-to-change link and the start hooks.
- Orca is found the way its guide prescribes: `$ORCA_CLI_COMMAND`, then
  `orca-ide` on Linux (a bare `orca` there is the GNOME screen reader), then
  `orca`.
- After Orca returns, `start` checks the worktree is on the exact branch it
  asked for, because Orca renames a taken branch (`-2`) instead of failing.
- `start` removes any upstream Orca set on the new branch, so a bare `git
  push` cannot reach the base branch; this matches the git launcher's
  `--no-track`.
- `-dry-run` prints the Orca command.

## Capabilities

### New Capabilities

- `workspace-start-orca`: `start` creates the worktree through Orca when
  configured.

## Impact

- `start.go` (`StartWork`), `cmd/specsync/cmd_start.go`, `ways.go` (config
  key), `cmd/specsync/orca.go` (already on main).
- README, `docs/config-specsync-example.yaml`, `site/features.json`, skill.

## Release note

`specsync start` can create the change's worktree as an Orca workspace,
linked to its issue (`worktrees.launcher: orca`).
