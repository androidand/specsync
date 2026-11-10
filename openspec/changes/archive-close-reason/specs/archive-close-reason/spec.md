# archive-close-reason

## ADDED Requirements

### Requirement: Choose why a change is closed
`specsync archive` SHALL accept `-reason completed|not-planned|duplicate`, default `completed`, and SHALL
close the issue with that reason through the provider.

#### Scenario: Default
- **WHEN** `archive` runs without `-reason`
- **THEN** the issue closes as completed, as before

#### Scenario: Not planned with unchecked tasks
- **WHEN** `archive -reason not-planned` runs on a change with unchecked tasks
- **THEN** it archives without `-force` and the issue closes as not planned

### Requirement: Duplicate names its original
`-reason duplicate` SHALL require `-duplicate-of <ref>` and SHALL reject `-duplicate-of` with any other reason.

#### Scenario: Missing original
- **WHEN** `-reason duplicate` is given without `-duplicate-of`
- **THEN** `archive` refuses and writes nothing

### Requirement: Optional closing comment
`-comment <text>` SHALL be posted with the close. If the comment cannot be posted, the issue SHALL NOT be closed.

#### Scenario: Comment fails
- **WHEN** the provider cannot post the comment
- **THEN** the issue stays open and `archive` reports the error

### Requirement: Closed issues are not silently skipped
When the issue is already closed and a reason other than completed, or a comment, was given, the archive SHALL fail instead of reporting success.

#### Scenario: Already closed
- **WHEN** `archive -reason not-planned` runs against an issue that is already closed
- **THEN** the archive fails, names the issue, and the change stays in place
