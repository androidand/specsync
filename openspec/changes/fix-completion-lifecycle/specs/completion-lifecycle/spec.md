# Completion lifecycle requirements

## Requirement: One-pass completion

specsync SHALL derive lifecycle stage from the task state that exists after
inbound reconciliation in the same sync invocation.

### Scenario: Final task completed in GitHub

- **GIVEN** an active change with one unchecked local task
- **AND** the linked GitHub issue has that task checked
- **WHEN** sync runs with reconciliation enabled
- **THEN** `tasks.md` is checked locally
- **AND** the issue receives `stage:complete` in that same run
- **AND** the issue closes in that same run when `-close-completed` is enabled

## Requirement: Reversible completion state

When `-close-completed` is enabled, tracker open/closed state SHALL follow the
current derived lifecycle rather than only transitioning toward closed.

### Scenario: New work appears after completion

- **GIVEN** a completed change whose tracker item is closed
- **WHEN** an unchecked task is added locally and sync runs
- **THEN** the tracker item receives `stage:active`
- **AND** the tracker item is reopened

### Scenario: Archived change

- **GIVEN** a change under `changes/archive/`
- **WHEN** sync runs
- **THEN** its tracker item remains closed regardless of `-close-completed`

## Requirement: Explicit stage override

A non-empty `.status` value SHALL remain authoritative over task-derived stage.
The close behavior for an explicit `complete` value SHALL be documented and
covered by tests.
