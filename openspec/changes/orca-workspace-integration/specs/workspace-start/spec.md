# workspace-start

## ADDED Requirements

### Requirement: Start a synced change in its own worktree
specsync SHALL provide `start <slug>`, which creates or reuses a worktree for the
change on branch `feat/<issue>-<slug>`, branched from the fetched remote default
branch, and sets the change's stage to `active`. It SHALL exit after the
worktree exists and SHALL NOT supervise any agent.

#### Scenario: Starting a change with a linked issue
- **WHEN** change `fix-login-race` is linked to issue 42 and `specsync start fix-login-race` runs
- **THEN** a worktree exists on branch `feat/42-fix-login-race` based on the remote default branch
- **AND** the change's stage is `active`
- **AND** the tracker is not synced unless `-sync` was passed

#### Scenario: Starting a change that has no issue yet
- **WHEN** the change has never been synced
- **THEN** `start` fails without creating a branch or worktree
- **AND** the error names the sync command to run first

#### Scenario: Starting a change that already has a worktree
- **WHEN** a worktree on the change's branch already exists
- **THEN** `start` reports its path and creates nothing

#### Scenario: Previewing a start
- **WHEN** `start` runs with `-dry-run`
- **THEN** it prints the branch, worktree path and launcher command
- **AND** creates no branch, worktree or stage change

### Requirement: Launch through an orchestrator when one is available
`start` SHALL accept `-launcher auto|orca|git`. With `auto` it SHALL use Orca
when an Orca executable resolves and fall back to `git worktree add` otherwise.
The Orca executable SHALL be resolved from `$ORCA_CLI_COMMAND`, then `orca-ide`
on Linux, then `orca`; on Linux it SHALL NOT invoke a bare `orca`.

#### Scenario: Orca is installed
- **WHEN** `start` runs with `-launcher auto` and Orca resolves
- **THEN** it runs `orca worktree create` with the change slug as name, the branch, the default branch as base, and the issue number
- **AND** passes `-agent` and `-prompt` through when given

#### Scenario: Orca is not installed
- **WHEN** `start` runs with `-launcher auto` and no Orca executable resolves
- **THEN** it creates the worktree with git, at the same path `pull -worktree` would use

#### Scenario: Linux without Orca's CLI
- **WHEN** the platform is Linux, `$ORCA_CLI_COMMAND` is unset and only `/usr/bin/orca` exists
- **THEN** `start` treats Orca as unavailable and uses git

### Requirement: Infer the change from the current branch
specsync SHALL let commands that act on one change (`set-stage`, `archive`,
`note`, and the default sync) resolve the change from the current branch when no change is
given and the branch matches `feat/<issue>-<slug>`. An explicit change SHALL
take precedence. On any other branch they SHALL fail as they do today.

#### Scenario: Setting the stage from inside a change worktree
- **WHEN** the current branch is `feat/42-fix-login-race` and `specsync set-stage active` runs
- **THEN** change `fix-login-race` moves to `active`

#### Scenario: Running on the default branch
- **WHEN** the current branch is `main` and `specsync set-stage active` runs
- **THEN** the command fails with "no change given" and changes nothing
