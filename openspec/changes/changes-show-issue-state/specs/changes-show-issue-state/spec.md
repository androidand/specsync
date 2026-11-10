# changes-show-issue-state

## ADDED Requirements

### Requirement: Show issue state on request
`specsync changes -issues` SHALL resolve each change's issue from the ref cache, fetch the states in one
batched provider call, and add the issue number and state to the table and to `-json`. Without `-issues`
it SHALL make no provider call and its output SHALL be unchanged.

#### Scenario: Without the flag
- **WHEN** `specsync changes` runs without `-issues`
- **THEN** no provider call is made and the output matches today's

#### Scenario: With the flag
- **WHEN** `specsync changes -issues` runs and a change has a cached issue ref
- **THEN** its row shows the issue number and `open` or `closed (<reason>)`

#### Scenario: Change without an issue
- **WHEN** a change has no cached issue ref
- **THEN** its issue columns are empty and it never reports drift
- **AND** the output ends with the number of changes that could not be checked

### Requirement: Flag drift between change and issue
With `-issues`, specsync SHALL flag `closed-unarchived` when the issue is closed and the change is not
archived, and `complete-open` when the change has at least one task, every task is checked and the issue is open. It SHALL flag nothing else.

#### Scenario: Closed elsewhere
- **WHEN** an issue was closed outside specsync and its change is still in `openspec/changes/`
- **THEN** the change is flagged `closed-unarchived`

#### Scenario: No tasks
- **WHEN** a change has no task checkboxes and its issue is open
- **THEN** it is not flagged `complete-open`

#### Scenario: Unchecked tasks
- **WHEN** a change has unchecked tasks and its issue is open
- **THEN** it is not flagged

### Requirement: Drift as an exit status
`specsync changes -issues -drift` SHALL list only changes with drift and exit non-zero when any exist.

#### Scenario: Clean repo
- **WHEN** no change has drift
- **THEN** the command prints nothing and exits zero

#### Scenario: No cache
- **WHEN** no change has a cached ref (a fresh checkout)
- **THEN** `-drift` exits zero and the unchecked count is printed to stderr
