# branch-change-inference

## ADDED Requirements

### Requirement: Resolve the change from the current branch
specsync SHALL resolve a change from the current branch by taking the branch's last path segment, using `<rest>` when the segment is `<digits>-<rest>` and `<rest>` names an active change, and otherwise using the segment itself when it names an active change. It SHALL resolve nothing for the default branch, a detached HEAD, an archived change, or a segment that names no change.

#### Scenario: This repo's branch convention
- **WHEN** the current branch is `feat/42-fix-login-race` and change `fix-login-race` exists
- **THEN** the resolved change is `fix-login-race`

#### Scenario: An Orca workspace named after the change
- **WHEN** the current branch is `someuser/fix-login-race` and change `fix-login-race` exists
- **THEN** the resolved change is `fix-login-race`

#### Scenario: The default branch
- **WHEN** the current branch is `main`
- **THEN** nothing is resolved

#### Scenario: A branch for an archived change
- **WHEN** the current branch is `feat/42-old-change` and `old-change` is archived
- **THEN** nothing is resolved

### Requirement: Single-change commands use the resolved change
`set-stage` with one argument, `note` with one argument, and `archive` without `-change` SHALL act on the resolved change and SHALL report the change and branch they picked on stderr. An explicitly named change SHALL always take precedence. When nothing resolves they SHALL fail without changing anything. Bare `specsync` SHALL keep syncing every change.

#### Scenario: Setting the stage from a change worktree
- **WHEN** the current branch is `feat/42-fix-login-race` and `specsync set-stage active` runs
- **THEN** change `fix-login-race` moves to `active`
- **AND** stderr names `fix-login-race` and the branch

#### Scenario: Setting the stage on the default branch
- **WHEN** the current branch is `main` and `specsync set-stage active` runs
- **THEN** the command fails and no change's metadata is written

#### Scenario: An explicit change wins
- **WHEN** the current branch is `feat/42-fix-login-race` and `specsync set-stage other-change active` runs
- **THEN** `other-change` moves to `active` and `fix-login-race` is untouched
