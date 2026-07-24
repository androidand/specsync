# audit-tasks

## ADDED Requirements

### Requirement: Scan changes for task-drift
specsync SHALL provide an `audit-tasks` command that loads all OpenSpec changes,
counts unchecked vs. total tasks from tasks.md, and flags mismatches where code
evidence exists but tasks remain unchecked.

#### Scenario: No mismatches
- **WHEN** `specsync audit-tasks` runs and all changes have either checked tasks
  or no code evidence
- **THEN** it outputs a table of all changes with task counts
- **AND** it exits with status 0

#### Scenario: Mismatches detected
- **WHEN** `specsync audit-tasks` runs and some changes have unchecked tasks with
  code evidence
- **THEN** it outputs a table with a "CODE" indicator for mismatched changes
- **AND** it prints a summary of mismatches

#### Scenario: Fail-fast mode
- **WHEN** `specsync audit-tasks -fail-on-mismatch` runs and mismatches exist
- **THEN** it exits with a non-zero status code
- **AND** the error message lists the mismatched changes

### Requirement: Detect implementation evidence
specsync SHALL detect implementation evidence via `.specsync/metadata.json` with
stage `complete` or `implemented`. This is the only reliable signal — design docs
contain Go type stubs during planning without actual implementation.

#### Scenario: Metadata stage complete
- **WHEN** a change has `.specsync/metadata.json` with `"stage":"complete"`
- **THEN** `hasImplementationEvidence` returns true

#### Scenario: Metadata stage implemented
- **WHEN** a change has `.specsync/metadata.json` with `"stage":"implemented"`
- **THEN** `hasImplementationEvidence` returns true

#### Scenario: Design doc only (no code)
- **WHEN** a change has `design.md` with Go type stubs but metadata stage is not
  complete or implemented
- **THEN** `hasImplementationEvidence` returns false

### Requirement: Provide machine-readable audit output
specsync SHALL offer `specsync audit-tasks -json` that outputs the audit findings
as structured JSON, suitable for CI pipelines and programmatic consumption.

#### Scenario: JSON output for CI
- **WHEN** `specsync audit-tasks -json` runs
- **THEN** the output is valid JSON with a `findings` array
- **AND** each finding has `slug`, `unchecked`, `total`, `hasCode`, `progress`, and `stage` fields
