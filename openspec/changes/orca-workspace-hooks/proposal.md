# Keep changes in step with Orca workspaces

## Why

Orca users mostly start work from the tracker: pick an issue in Orca's GitHub,
Linear or Projects view and create a workspace for it. Orca hands the agent the
issue as its brief. With specsync that issue *is* the spec, rendered from
`proposal.md` and `tasks.md`, so the two agree on what the work is — but
nothing keeps the OpenSpec side in step with the workspace:

- A workspace made from an issue that has no local change yet never becomes a
  change, so the agent works from the issue body alone and the commits have no
  change to bind to in the changelog.
- A workspace made from an issue that *does* have a change is on Orca's
  `<user>/<workspace-name>` branch, which may not name the change.
- The change's stage stays wherever it was when the workspace opens and
  closes.

Orca runs repository hooks from `orca.yaml` (`scripts.setup` when a workspace
is created, `scripts.archive` when it is archived). Those hooks get
`ORCA_WORKTREE_PATH`, `ORCA_WORKTREE_ID` and `ORCA_ROOT_PATH`, not the linked
issue; `orca worktree current --json` reports the workspace's linked issue.

Depends on `infer-change-from-branch`.

## What Changes

- New `specsync workspace setup`, meant to run from a workspace hook:
  1. Find the change from the branch; failing that, from the workspace's
     linked issue (via `orca worktree current --json` when running under
     Orca); failing that, do nothing and exit 0, so an unrelated workspace is
     never blocked.
  2. If the linked issue has no local change, pull it into one
     (`specsync pull -issue <n>`), so issue-first work becomes a change.
  3. Set the change's stage to `active`.
- New `specsync workspace archive`: finds the change the same way and sets its
  stage back to `auto` (derived from tasks), so an abandoned workspace does
  not leave the change stuck `active`.
- Both print what they did and never sync the tracker; the agent or a human
  syncs as usual.
- Document an `orca.yaml` recipe:

  ```yaml
  scripts:
    setup: |
      specsync workspace setup
    archive: |
      specsync workspace archive
  ```

- Document one writer for tracker status: Orca's opt-in Linear status sync and
  specsync's stage projection both move issues; enable one, not both.
- Narrow `in-flight-work-claims` to what an orchestrator does not cover:
  whether a squash-merged change has landed, and overlapping files across
  worktrees. Drop same-worktree and volatile-path detection, which
  per-workspace orchestration prevents.

## Capabilities

### New Capabilities

- `orca-workspace-hooks`: workspace lifecycle hooks that turn a workspace into
  a change and keep its stage in step.

## Impact

- New `workspace` subcommand in `cmd/specsync/`, registered in `agent-help`.
- Shells out to Orca only when it resolves (same resolution as `start`).
- README "Working with Orca" section, `site/features.json`, specsync skill.
- Edits `openspec/changes/in-flight-work-claims/`.

## Release note

`specsync workspace setup` and `workspace archive` keep a change in step with
an Orca workspace, turning a workspace started from an issue into a change.
