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
  1. Find the change from the branch. Failing that, ask Orca for the
     workspace's linked issue (`orca worktree current --json`, field
     `result.worktree.linkedIssue`, a GitHub issue number or null) and find
     the change linked to it: first a local ref cache, then the issue's
     `specsync:change=` marker naming a local change. A fresh worktree has no
     ref caches (they are gitignored), so the marker step is what finds a
     committed change there; it binds the cache with `adopt`, not `pull`,
     because `pull` would make a suffixed copy and overwrite the proposal.
     Orca is only queried when the branch names no change.
  2. If no local change is linked to the issue, pull it into one, so
     issue-first work becomes a change.
  3. Set the change's stage to `active`.
  Failing all of that it does nothing and exits 0, so an unrelated workspace
  is never blocked.
- New `specsync workspace archive`: finds the change the same way (without
  pulling) and sets its stage back to `auto`, derived from tasks.
- Both sync the tracker only with `-sync`, and accept `-dry-run`. The stage is
  kept per checkout in gitignored metadata, and Orca removes the worktree on
  archive, so `workspace archive` only has an effect with `-sync`; the recipe
  uses it there.
- Document an `orca.yaml` recipe:

  ```yaml
  scripts:
    setup: |
      specsync workspace setup
    archive: |
      specsync workspace archive -sync
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
- Shells out to Orca only when it resolves (same resolution as `start`,
  shared in `cmd/specsync/orca.go`).
- Orca's `linkedIssue` is a GitHub issue number; Linear and GitLab links are
  separate fields and are not read.
- README "Working with Orca" section, `site/features.json`, specsync skill.
- Edits `openspec/changes/in-flight-work-claims/`.

## Release note

`specsync workspace setup` and `workspace archive` keep a change in step with
an Orca workspace, turning a workspace started from an issue into a change.
