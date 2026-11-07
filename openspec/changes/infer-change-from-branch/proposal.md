# Find the change from the branch you are on

## Why

An agent working inside a change's worktree still has to name the change on
every command: `specsync set-stage my-change active`, `specsync note
my-change "..."`, `specsync archive -change my-change`. The branch already
says which change it is. Making the agent repeat it invites typos and lets an
agent act on the wrong change from the wrong checkout.

It also blocks hook-driven workflows. Workspace orchestrators such as Orca run
repository hooks (`orca.yaml` `scripts.setup` / `scripts.archive`) when a
worktree is created or archived. The hooks get the worktree path but not the
change, so a hook cannot call specsync unless specsync can work the change out
for itself.

Two branch conventions are in use and both must work:

- this repo's `feat/<issue>-<slug>`;
- Orca's default `<prefix>/<workspace-name>`, where the prefix is the git user
  name, so a workspace named after the change gives `<user>/<slug>`.

## What Changes

- Resolve the change from the current branch: take the branch's last path
  segment; if it looks like `<digits>-<rest>` and `<rest>` is a change, use it;
  otherwise use the segment itself if it is a change. Only active (not
  archived) changes count. A branch that matches nothing, the default branch,
  and a detached HEAD resolve to nothing.
- `set-stage <stage>`, `note <text>` and `archive` (without `-change`) use the
  resolved change, and print which change and branch they picked on stderr.
  An explicit change always wins. If nothing resolves, they fail as today.
- Bare `specsync` (sync everything) is unchanged: inferring a single change
  there would silently narrow what a sync does.

## Capabilities

### New Capabilities

- `branch-change-inference`: commands that act on one change find it from the
  current branch.

## Impact

- New resolver in `cmd/specsync/`, used by `runSetStage`, `runNote` and
  `runArchive`; `agent-help` entries updated.
- README and `site/features.json` gain a short note.

## Release note

Commands that act on one change (`set-stage`, `note`, `archive`) now find it
from the branch you are on, including Orca's `<user>/<workspace>` branches.
