# board-status-reconcile

## ADDED Requirements

### Requirement: Exact specsync-managed status tracking

specsync SHALL persist the board status option it last wrote per change and
SHALL treat a board Status as specsync-managed only when it equals that
persisted value — never by matching against the set of names specsync could
have written.

#### Scenario: Human move to a managed-sounding name is respected

- **GIVEN** specsync last wrote "In Progress" for a change
- **AND** a human moves the card to "Done"
- **WHEN** the next sync runs while the change is still active
- **THEN** the Status is left at "Done" and the move is surfaced in the report

### Requirement: Human board moves reconcile inbound

A human board move SHALL be treated as signal: a move to a Done-like option
while local tasks are incomplete is surfaced (never silently reverted); a move
to an active option while the local stage is complete is treated as a reopen,
consistent with `-close-completed` reopen semantics.

#### Scenario: Board reopen propagates

- **GIVEN** a change whose tasks are all checked and whose card sits in "Done"
- **AND** a human drags the card back to "In Progress"
- **WHEN** the next sync runs
- **THEN** the reopen is reported and the issue/stage lifecycle follows the
  same rules as a reopened issue
