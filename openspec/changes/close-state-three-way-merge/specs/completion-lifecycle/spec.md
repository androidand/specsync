# completion-lifecycle

## MODIFIED Requirements

### Requirement: Reversible completion state
specsync SHALL reverse a closed tracker item only when that close was its own last
assertion. It SHALL record, per ref, the open/closed state it last asserted as a
merge base, and reopen only when the base says closed and the current derived
lifecycle says open. A base of open — or no recorded base at all — SHALL mean the
close came from outside specsync, and specsync SHALL leave the item's open/closed
state untouched. An externally closed item SHALL NOT have its close adopted as a
new base, so specsync remains deferential on later runs rather than re-arming
itself to reopen.

#### Scenario: New work appears after specsync closed the item
- **GIVEN** a completed change whose tracker item specsync closed
- **WHEN** an unchecked task is added locally and sync runs
- **THEN** the tracker item receives `stage:active`
- **AND** the tracker item is reopened

#### Scenario: A merged PR closed the issue
- **GIVEN** a change whose tracker item specsync last left open
- **AND** the item was then closed outside specsync — by a merged PR, a human, or a reviewing agent
- **WHEN** a later sync runs with `-close-completed` and the change is not complete
- **THEN** the item stays closed
- **AND** its body and labels are still updated

#### Scenario: A discarded ref cache does not license a reopen
- **GIVEN** a closed tracker item and a ref cache with no recorded open/closed base
- **WHEN** sync runs with `-close-completed` and the change is not complete
- **THEN** the item stays closed

#### Scenario: Deferring does not re-arm specsync
- **GIVEN** specsync has deferred to an external close
- **WHEN** sync runs again
- **THEN** the item is still not reopened
- **AND** no open/closed base is invented for it

#### Scenario: A re-pull does not reset the base
- **GIVEN** a change whose ref records an open/closed base
- **WHEN** the issue is pulled again
- **THEN** the recorded base is carried forward unchanged
- **AND** it is not re-seeded from the issue's current open/closed state

## ADDED Requirements

### Requirement: Pull records the task merge base it creates
specsync SHALL record a task merge base whenever `pull` writes `tasks.md` from an
issue body, using the content it wrote. Pull makes local and remote identical,
which is exactly what a merge base records, so the pulled content — not a carried
over earlier base — is the correct base. When the issue carries no `## Tasks`
section, `tasks.md` is left untouched and the prior base SHALL be preserved rather
than dropped. Without a base, the next sync degrades to a monotonic union in which
a task unchecked on the issue can never propagate back.

#### Scenario: Pull seeds the base from what it wrote
- **WHEN** an issue with a `## Tasks` section is pulled
- **THEN** the recorded base equals the `tasks.md` content pull wrote
- **AND** the recorded base checksum matches that content

#### Scenario: An uncheck propagates after a pull
- **GIVEN** a change pulled from an issue with a checked task
- **WHEN** that task is unchecked on the issue and sync runs with reconcile
- **THEN** the uncheck propagates into `tasks.md`

#### Scenario: A task-less pull keeps the prior base
- **WHEN** an issue with no `## Tasks` section is pulled over an existing change
- **THEN** `tasks.md` is unchanged
- **AND** the previously recorded task base and checksum survive

#### Scenario: Archived change
- **GIVEN** a change under `changes/archive/`
- **WHEN** sync runs
- **THEN** its tracker item remains closed regardless of `-close-completed`

#### Scenario: Closing records the base
- **GIVEN** a change whose every task is checked and an open tracker item
- **WHEN** sync runs with `-close-completed`
- **THEN** the item closes
- **AND** the recorded base is closed, licensing a later reopen

#### Scenario: Without the flag, open/closed state is never touched
- **GIVEN** any change and a closed tracker item
- **WHEN** sync runs without `-close-completed`
- **THEN** the item is neither closed nor reopened

## ADDED Requirements

### Requirement: Continuous integration does not decide closure
The repository's own spec-sync workflow SHALL NOT pass `-close-completed`. Closing
a tracker item is a judgement about whether work is done and reviewed and SHALL
rest with whoever merges the change — human or agent — rather than with a
path-filtered push. Automated syncing SHALL be limited to keeping item content
(title, body, task checklist, labels) current.

#### Scenario: A spec push does not close issues
- **WHEN** a push to the default branch touches `openspec/**` and the sync workflow runs
- **THEN** issue bodies, checklists, and labels are updated
- **AND** no issue is closed or reopened by that run

#### Scenario: Closing remains available on demand
- **WHEN** an operator runs sync locally with `-close-completed`
- **THEN** completion-driven closing behaves as specified above
