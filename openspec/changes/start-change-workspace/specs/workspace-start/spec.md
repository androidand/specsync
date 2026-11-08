# workspace-start

## ADDED Requirements

### Requirement: Start a synced change in its own worktree
specsync SHALL provide `start <slug>`, which creates or reuses a worktree for the change on branch `feat/<issue>-<slug>`, branched from the fetched remote default branch, and sets the change's stage to `active`. It SHALL exit once the worktree exists and SHALL NOT supervise any agent.

#### Scenario: Starting a change with a linked issue
- **WHEN** change `fix-login-race` is linked to issue 42 and `specsync start fix-login-race` runs
- **THEN** a worktree exists on branch `feat/42-fix-login-race` based on the remote default branch
- **AND** the change's stage is `active`
- **AND** the tracker is not synced unless `-sync` was passed

#### Scenario: Starting a change that has no issue yet
- **WHEN** the change has never been synced
- **THEN** `start` fails without creating a branch or worktree and names the sync command to run

#### Scenario: Starting a change that already has a worktree
- **WHEN** a worktree on the change's branch already exists
- **THEN** `start` reports its path and creates nothing

#### Scenario: Previewing a start
- **WHEN** `start` runs with `-dry-run`
- **THEN** it prints the branch, worktree path and launcher command and changes nothing

### Requirement: Launch through Orca when it is installed
`start` SHALL accept `-launcher auto|orca|git`. With `auto` it SHALL use Orca when an Orca executable resolves and git otherwise. It SHALL resolve Orca from `$ORCA_CLI_COMMAND`, then `orca-ide` on Linux, then `orca`, and SHALL NOT invoke a bare `orca` on Linux.

#### Scenario: Orca is installed
- **WHEN** `start` runs with `-launcher auto` and Orca resolves
- **THEN** it runs `orca worktree create` with the slug as name, the branch, the default branch as base and the issue number
- **AND** passes `-agent` and `-prompt` through when given

#### Scenario: Orca is not installed
- **WHEN** no Orca executable resolves
- **THEN** `start` creates the worktree with git at the path `pull -worktree` uses

#### Scenario: Linux with only the screen reader
- **WHEN** the platform is Linux, `$ORCA_CLI_COMMAND` is unset and `orca-ide` is not on the path
- **THEN** `start` treats Orca as unavailable and uses git
