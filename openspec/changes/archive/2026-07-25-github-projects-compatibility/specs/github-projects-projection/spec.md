# github-projects-projection

## ADDED Requirements

### Requirement: Opt-in target project
specsync SHALL accept an optional target GitHub ProjectV2 for `sync` and `pull`,
given as `owner/number` (org or user), via a `-project` flag and/or persisted
config. When no target project is configured, specsync SHALL perform no board
operations and behave exactly as it does today.

#### Scenario: Unconfigured is a no-op
- **WHEN** `sync` runs with no `-project` and no configured project
- **THEN** no ProjectV2 query or mutation is issued
- **AND** issue create/update behaves exactly as before

#### Scenario: Target project resolves to a node id
- **WHEN** `-project org/1` is given
- **THEN** specsync resolves the project's node id once and reuses it for the run

### Requirement: Resolve the Status field and options from the project schema
specsync SHALL discover the project's single-select **Status** field id and its
option ids by querying the project schema (matching the field by name), and SHALL
map a status name to an option id with a fail-loud error listing the valid options
when the name is unknown. Field/option ids SHALL NOT be hard-coded.

#### Scenario: Status name maps to an option id
- **WHEN** the stage maps to status name "In progress" and the board defines that option
- **THEN** specsync resolves it to that option's id for the field update

#### Scenario: Unknown status fails loudly
- **WHEN** a configured status name does not exist on the board
- **THEN** specsync errors, listing the board's valid Status options
- **AND** it does not silently skip the update

### Requirement: Detect board membership before acting
specsync SHALL determine whether the change's issue is already an item of the target
project by inspecting the issue's project items, distinguishing "in the repo but not
on the board" from "already on the board", and SHALL use this to stay idempotent.

#### Scenario: Freshly created issue is not yet on the board
- **WHEN** specsync creates the issue and a target project is configured
- **THEN** membership detection reports it absent
- **AND** specsync adds it to the board

#### Scenario: Re-running does not duplicate the board item
- **WHEN** the issue is already an item of the target project
- **THEN** specsync does not add it again
- **AND** it reuses the existing project item id for field updates

### Requirement: Ensure the issue is on the board, map stage to Status, and assign
With a target project configured, specsync SHALL add the synced change's issue to
the board when absent (`addProjectV2ItemById` with the issue content id), set the
board **Status** from the change's stage (`updateProjectV2ItemFieldValue` with the
resolved `singleSelectOptionId` on the project item id), and set an assignee — the
acting viewer by default (`"me"`), or a configured assignee, resolved login → user
id.

#### Scenario: A synced change appears as active work on the board
- **WHEN** an active change is synced with `-project org/1`
- **THEN** its issue is on the board, its Status is the mapped active option, and it is assigned
- **AND** it no longer appears only as a `stage:active`-labelled issue off the board

#### Scenario: Stage maps to Status
- **WHEN** a change's stage is `archived`
- **THEN** the board Status is set to the mapped terminal option (e.g. "Done")

### Requirement: Never clobber human board curation
specsync SHALL NOT overwrite a Status or assignee that a human set: it sets Status
only when unset or when the current value is one specsync last wrote, sets an
assignee only when none exists (never removing existing assignees), and SHALL NEVER
remove an item from the board.

#### Scenario: A human-moved card is respected
- **WHEN** the issue's board Status was changed by a person since the last sync
- **THEN** specsync leaves that Status unchanged

#### Scenario: Existing assignees are preserved
- **WHEN** the issue already has an assignee
- **THEN** specsync does not replace or remove it

### Requirement: Require and report the project token scope
When board operations are requested, specsync SHALL surface a clear error stating
the `project` scope is required if the token lacks it, rather than presenting a raw
GraphQL/permission error.

#### Scenario: Missing project scope is explained
- **WHEN** a board mutation is rejected for insufficient scope
- **THEN** specsync reports that the token needs the `project` scope
- **AND** names the operation that failed

### Requirement: Dry-run previews board changes without writing
`-dry-run` SHALL print the board plan it would apply — add item, set Status option,
set assignee — and SHALL make no ProjectV2 mutations.

#### Scenario: Preview the board plan
- **WHEN** `sync -dry-run -project org/1` runs for an off-board change
- **THEN** it prints that it would add the issue, set Status, and assign it
- **AND** no GraphQL mutation is sent
