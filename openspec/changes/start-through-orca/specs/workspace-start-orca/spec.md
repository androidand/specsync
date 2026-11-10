# workspace-start-orca

## ADDED Requirements

### Requirement: Create the worktree through Orca when configured
`specsync start` SHALL create a new worktree with `orca worktree create` when the launcher is `orca`, or `auto` and Orca resolves, passing the change slug as the name, the configured branch, the base, `--no-parent`, and the change's issue when it has one. The default launcher SHALL be `git`, leaving today's behaviour unchanged. It SHALL resolve Orca from `$ORCA_CLI_COMMAND`, then `orca-ide` on Linux, then `orca`, and SHALL NOT run a bare `orca` on Linux.

#### Scenario: Orca configured and installed
- **WHEN** `worktrees.launcher` is `orca` and change `add-retry` is linked to issue 42
- **THEN** `start add-retry` runs `orca worktree create --name add-retry --branch feat/42-add-retry --base-branch <base> --no-parent --issue 42`
- **AND** records the branch-to-change link and runs the start hooks in the new worktree

#### Scenario: Linux with only the screen reader
- **WHEN** the launcher is `auto` on Linux, `$ORCA_CLI_COMMAND` is unset and `orca-ide` is not installed
- **THEN** `start` creates the worktree with git

#### Scenario: Orca explicitly requested but missing
- **WHEN** the launcher is `orca` and Orca does not resolve
- **THEN** `start` fails before creating anything

### Requirement: Keep Orca's worktree on the expected branch, unpublished
After Orca creates the worktree, `start` SHALL verify a worktree exists on the exact branch it asked for and SHALL fail naming the branch otherwise. It SHALL remove any upstream configured on the new branch.

#### Scenario: Orca renames a taken branch
- **WHEN** Orca creates the workspace on `feat/42-add-retry-2` instead of `feat/42-add-retry`
- **THEN** `start` fails, naming the branch it expected

#### Scenario: Orca set an upstream
- **WHEN** the branch Orca created tracks `origin/main`
- **THEN** after `start` the branch has no upstream
