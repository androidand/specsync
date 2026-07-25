# Spec: Rich Change State Model

## Requirement: Task progress is distinct from workflow stage

specsync SHALL track task progress (what checkboxes say) separately from workflow stage (where humans place work).

### Scenario: Complete stage with incomplete tasks

- **GIVEN** a change with 10 tasks, 5 complete
- **AND** `.specsync.yaml` stage = complete
- **WHEN** specsync loads the change
- **THEN** stage = complete, progress = in-progress
- **AND** stageSource = metadata
- **AND** the stage is explicit user intent, not derived

### Scenario: Active stage with all tasks done

- **GIVEN** a change with 5 tasks, all complete
- **AND** `.specsync.yaml` stage = active
- **WHEN** specsync loads the change
- **THEN** stage = active, progress = complete
- **AND** the stage is workflow placement, not task status

### Scenario: Progress value matches task state

- **GIVEN** changes with varying task completion
- **WHEN** specsync loads each
- **THEN** progress reflects task checklist:
  - no tasks.md → no-tasks
  - 0/5 tasks checked → not-started
  - 2/5 tasks checked → in-progress
  - 5/5 tasks checked → complete

## Requirement: Six canonical stages

specsync SHALL recognize and validate six standard workflow stages: backlog, blocked, active, in-review, complete, archived.

### Scenario: Canonical stage values are valid

- **GIVEN** a change with `.specsync.yaml` stage = blocked
- **WHEN** specsync loads the change
- **THEN** stage = blocked
- **AND** stageSource = metadata
- **AND** no error occurs

### Scenario: Stage constants exist with correct names

- **GIVEN** the Stage enum
- **THEN** six constants exist: StageBacklog, StageBlocked, StageActive, StageInReview, StageComplete, StageArchived
- **AND** string values match: "backlog", "blocked", "active", "in-review", "complete", "archived"

## Requirement: Custom stages are valid with constraints

specsync SHALL accept custom stage values that match the pattern `^[a-z0-9][a-z0-9-]{0,63}$`.

### Scenario: Valid custom stage

- **GIVEN** a `.status` file or `.specsync.yaml` with stage = qa-ready
- **WHEN** specsync loads the change
- **THEN** stage = qa-ready
- **AND** stageSource reports the source (legacy-status or metadata)
- **AND** canonicalStage = false in JSON output

### Scenario: Invalid custom stage is rejected

- **GIVEN** `.specsync.yaml` with stage = "Waiting for Bob!!!"
- **WHEN** specsync loads the change
- **THEN** an error is returned
- **AND** the error message names the invalid stage and the pattern

### Scenario: Custom stage position in ordering

- **GIVEN** changes with stages: backlog, qa-ready, active, archived
- **WHEN** sorted by canonical order (backlog → blocked → active → in-review → complete → archived, custom alphabetically)
- **THEN** order is: backlog, active, qa-ready, archived

## Requirement: Stage source is tracked

specsync SHALL record how each stage was derived via StageSource.

### Scenario: Default source

- **GIVEN** a change with no .specsync.yaml, no .status, unchecked tasks
- **WHEN** specsync loads the change
- **THEN** stage = active
- **AND** stageSource = default

### Scenario: Tasks source

- **GIVEN** a change with no .specsync.yaml, no .status, all tasks complete
- **WHEN** specsync loads the change
- **THEN** stage = complete
- **AND** stageSource = tasks

### Scenario: Metadata source

- **GIVEN** a change with `.specsync.yaml` stage = blocked
- **WHEN** specsync loads the change
- **THEN** stageSource = metadata

### Scenario: Legacy-status source

- **GIVEN** a change with `.status` file containing blocked
- **AND** no `.specsync.yaml`
- **WHEN** specsync loads the change
- **THEN** stageSource = legacy-status

### Scenario: Folder source

- **GIVEN** a change under changes/archive/
- **WHEN** specsync loads the change
- **THEN** stage = archived
- **AND** stageSource = folder

## Requirement: Archived folder location is immutable

specsync SHALL treat archived folder location as final. No other stage source can override it.

### Scenario: .specsync.yaml stage is ignored for archived changes

- **GIVEN** a change under changes/archive/
- **AND** `.specsync.yaml` with stage = active
- **WHEN** specsync loads the change
- **THEN** stage = archived
- **AND** stageSource = folder
- **AND** the .specsync.yaml is not considered

### Scenario: Legacy .status is ignored for archived changes

- **GIVEN** a change under changes/archive/
- **AND** `.status` file with stage = blocked
- **WHEN** specsync loads the change
- **THEN** stage = archived
- **AND** no warning about the .status file

### Scenario: Archived changes reject mutation

- **GIVEN** an archived change
- **WHEN** `specsync set-stage` is called (new CLI)
- **THEN** an error is returned
- **AND** no file is written

## Requirement: .specsync.yaml schema and parsing

specsync SHALL load and validate `.specsync.yaml` with strict version checking and field validation.

### Scenario: Valid .specsync.yaml with all fields

- **GIVEN** a `.specsync.yaml` file:
  ```yaml
  version: 1
  stage: blocked
  priority: 5
  ```
- **WHEN** specsync loads the change
- **THEN** stage = blocked
- **AND** priority = 5

### Scenario: .specsync.yaml with only priority

- **GIVEN** a `.specsync.yaml` file:
  ```yaml
  version: 1
  priority: 3
  ```
- **WHEN** specsync loads the change
- **THEN** priority = 3
- **AND** stage is derived from other sources

### Scenario: Missing version is treated as v1

- **GIVEN** a `.specsync.yaml` file with no version field
- **WHEN** specsync loads the change
- **THEN** no error occurs
- **AND** file is treated as version 1

### Scenario: Unsupported version is rejected

- **GIVEN** a `.specsync.yaml` file with version: 2
- **WHEN** specsync loads the change
- **THEN** an error is returned
- **AND** error message names the unsupported version

### Scenario: Invalid YAML is reported clearly

- **GIVEN** a `.specsync.yaml` file with malformed YAML:
  ```yaml
  version 1
  stage: blocked
  ```
- **WHEN** specsync loads the change
- **THEN** an error is returned
- **AND** error message indicates YAML parse failure

## Requirement: Priority validation

specsync SHALL enforce priority values to be 1–100 when present.

### Scenario: Valid priority

- **GIVEN** `.specsync.yaml` with priority: 42
- **WHEN** specsync loads the change
- **THEN** priority = 42 (as *int)

### Scenario: Priority unset is null

- **GIVEN** `.specsync.yaml` with no priority field
- **WHEN** specsync loads the change
- **THEN** priority is nil (not 0)

### Scenario: Out-of-range priority is rejected

- **GIVEN** `.specsync.yaml` with priority: 150
- **WHEN** specsync loads the change
- **THEN** an error is returned
- **AND** error message states priority must be 1–100

### Scenario: Invalid priority in committed file fails

- **GIVEN** a `.specsync.yaml` with priority: not-a-number
- **WHEN** `specsync sync` runs
- **THEN** that change is skipped and an error is reported
- **AND** the invalid state is visible, not silently ignored

## Requirement: .specsync.yaml wins over .status when both exist

specsync SHALL prefer `.specsync.yaml` stage over `.status` when both are present.

### Scenario: Both files exist with different stages

- **GIVEN** a change with:
  - `.specsync.yaml` stage = active
  - `.status` file = blocked
- **WHEN** specsync loads the change
- **THEN** stage = active
- **AND** stageSource = metadata
- **AND** a warning is emitted to stderr

### Scenario: Warning includes file names

- **GIVEN** the above scenario
- **WHEN** specsync runs
- **THEN** stderr includes:
  - the change slug
  - both file names
  - which value is being used
  - hint to run `set-stage` to migrate

### Scenario: Both files agree, no warning

- **GIVEN** a change with:
  - `.specsync.yaml` stage = blocked
  - `.status` file = blocked
- **WHEN** specsync loads the change
- **THEN** no warning is emitted

## Requirement: Invalid metadata is visible

specsync SHALL fail clearly when committed metadata is malformed, rather than silently falling back.

### Scenario: Read failure in changes command

- **GIVEN** a change with invalid `.specsync.yaml`
- **WHEN** `specsync changes` runs (new CLI)
- **THEN** the change appears in output with diagnostics
- **AND** other changes are listed normally
- **AND** the command completes with exit code 0 (continues)

### Scenario: Read failure in sync command

- **GIVEN** a change with invalid `.specsync.yaml`
- **WHEN** `specsync sync` runs
- **THEN** that change is skipped
- **AND** an error is printed
- **AND** the command exits non-zero
- **AND** other changes are synced normally

### Scenario: Read failure in set-stage

- **GIVEN** a change with invalid `.specsync.yaml` (e.g., priority: banana)
- **WHEN** `specsync set-stage my-change active` runs
- **THEN** an error is returned
- **AND** no file is written
- **AND** error message indicates the file must be corrected first

## Requirement: OpenSpec format compatibility

The rich change state model SHALL not modify OpenSpec's native format.

### Scenario: .specsync.yaml is not parsed by OpenSpec

- **GIVEN** an OpenSpec change with `.specsync.yaml`
- **WHEN** `openspec list --json` runs
- **THEN** the `.specsync.yaml` file is not mentioned
- **AND** no error occurs

### Scenario: Task progress matches OpenSpec's task-derived status

- **GIVEN** an OpenSpec change with 5 tasks, 3 complete
- **WHEN** OpenSpec reports status via `openspec list --json`
- **AND** specsync loads progress from tasks.md
- **THEN** both report consistent completion numbers
