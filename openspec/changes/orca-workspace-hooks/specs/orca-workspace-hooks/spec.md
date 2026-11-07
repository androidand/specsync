# orca-workspace-hooks

## ADDED Requirements

### Requirement: Turn a new workspace into an active change
specsync SHALL provide `workspace setup`, which finds the workspace's change from the current branch or, failing that, from the workspace's linked issue as reported by Orca, pulls the linked issue into a new change when none exists locally, and sets the change's stage to `active`. When neither the branch nor a linked issue identifies work, it SHALL exit successfully and change nothing. It SHALL NOT sync the tracker.

#### Scenario: Workspace on a change's branch
- **WHEN** the branch is `someuser/fix-login-race` and change `fix-login-race` exists
- **THEN** `workspace setup` sets `fix-login-race` to `active`

#### Scenario: Workspace created from an issue that has a change
- **WHEN** the branch names no change and Orca reports linked issue 42, which belongs to change `fix-login-race`
- **THEN** `workspace setup` sets `fix-login-race` to `active`

#### Scenario: Workspace created from an issue with no change
- **WHEN** Orca reports linked issue 57 and no local change is linked to it
- **THEN** `workspace setup` pulls issue 57 into a new change and sets it to `active`

#### Scenario: Unrelated workspace
- **WHEN** the branch names no change and there is no linked issue
- **THEN** `workspace setup` exits 0 and writes nothing

### Requirement: Release the change when the workspace is archived
specsync SHALL provide `workspace archive`, which finds the change the same way as `workspace setup` and sets its stage to `auto`. When no change is found it SHALL exit successfully and change nothing.

#### Scenario: Archiving an abandoned workspace
- **WHEN** a workspace for change `fix-login-race` is archived with tasks still open
- **THEN** the change's stage is derived from its tasks again instead of staying `active`
