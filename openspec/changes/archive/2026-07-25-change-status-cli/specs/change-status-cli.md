# Spec: Change Status CLI

## Requirement: specsync changes lists local changes with state

specsync SHALL provide `specsync changes` to list OpenSpec changes with stage, priority, and progress.

### Scenario: List all changes

- **GIVEN** 8 changes across all stages
- **WHEN** `specsync changes` runs
- **THEN** all changes are displayed in a table
- **AND** output is grouped by canonical stage order
- **AND** within each stage, sorted by priority (1 first, unset last), then slug

### Scenario: Canonical stage order in output

- **GIVEN** changes with stages: archived, active, blocked, backlog, complete, in-review
- **WHEN** `specsync changes` runs
- **THEN** output order is: backlog, blocked, active, in-review, complete, archived

### Scenario: Default table columns

- **WHEN** `specsync changes` runs
- **THEN** table includes columns: STAGE, PRIORITY, SLUG, PROGRESS, TASKS, TITLE
- **AND** PRIORITY shows "-" for unset (not 0 or blank)
- **AND** PROGRESS shows task-derived value (no-tasks, not-started, in-progress, complete)
- **AND** TASKS shows "completed/total" (e.g. "3/8")

### Scenario: Filter by single stage

- **GIVEN** changes across stages
- **WHEN** `specsync changes -stage active` runs
- **THEN** only changes with stage=active appear
- **AND** other stages are not shown

### Scenario: Filter by multiple stages

- **GIVEN** changes across stages
- **WHEN** `specsync changes -stage backlog,blocked` runs
- **THEN** only changes with stage=backlog or stage=blocked appear
- **AND** order is still canonical (backlog first, then blocked)

### Scenario: Sort by priority

- **GIVEN** changes with priorities: 5, unset, 3, 1, unset
- **WHEN** `specsync changes -sort priority` runs
- **THEN** output order is: 1, 3, 5, unset, unset (sorted within each stage)

### Scenario: JSON output format

- **GIVEN** a change: slug=add-dark-mode, stage=backlog, priority=2, progress=not-started
- **WHEN** `specsync changes -json` runs
- **THEN** output includes JSON object with fields:
  - slug, title, stage, canonicalStage, stageSource, priority, taskProgress, completedTasks, totalTasks, archived, diagnostics
- **AND** priority is null (not 0) when unset
- **AND** canonicalStage is boolean
- **AND** diagnostics is array (may be empty)

### Scenario: JSON with diagnostics

- **GIVEN** a change with custom stage "qa-ready" and no board mapping
- **WHEN** `specsync changes -json` runs
- **THEN** diagnostics includes warning:
  ```json
  {
    "code": "unmapped-stage",
    "severity": "warning",
    "message": "Custom stage \"qa-ready\" has no GitHub Projects mapping"
  }
  ```

### Scenario: Filtered JSON

- **GIVEN** 8 changes, 3 with stage=active
- **WHEN** `specsync changes -json -stage active` runs
- **THEN** JSON array contains only 3 objects

## Requirement: set-stage mutates .specsync.yaml

specsync SHALL provide `specsync set-stage` to explicitly set a change's workflow stage.

### Scenario: Set stage in .specsync.yaml

- **GIVEN** a change with no .specsync.yaml
- **WHEN** `specsync set-stage my-change blocked` runs
- **THEN** .specsync.yaml is created with:
  ```yaml
  version: 1
  stage: blocked
  ```

### Scenario: Migrate from .status

- **GIVEN** a change with `.status` file = "active"
- **AND** no .specsync.yaml
- **WHEN** `specsync set-stage my-change blocked` runs
- **THEN** .specsync.yaml is created with stage=blocked
- **AND** .status file is deleted

### Scenario: Preserve priority when changing stage

- **GIVEN** a .specsync.yaml:
  ```yaml
  version: 1
  stage: active
  priority: 5
  ```
- **WHEN** `specsync set-stage my-change blocked` runs
- **THEN** .specsync.yaml becomes:
  ```yaml
  version: 1
  stage: blocked
  priority: 5
  ```
- **AND** priority is unchanged

### Scenario: set-stage auto removes explicit override

- **GIVEN** a .specsync.yaml with stage=blocked
- **WHEN** `specsync set-stage my-change auto` runs
- **THEN** stage field is removed from .specsync.yaml
- **AND** .status file is deleted if it exists

### Scenario: set-stage auto deletes empty .specsync.yaml

- **GIVEN** a .specsync.yaml:
  ```yaml
  version: 1
  stage: blocked
  ```
- **WHEN** `specsync set-stage my-change auto` runs
- **THEN** .specsync.yaml file is deleted entirely

### Scenario: set-stage auto preserves priority

- **GIVEN** a .specsync.yaml:
  ```yaml
  version: 1
  stage: blocked
  priority: 3
  ```
- **WHEN** `specsync set-stage my-change auto` runs
- **THEN** .specsync.yaml becomes:
  ```yaml
  version: 1
  priority: 3
  ```

### Scenario: Archived changes reject set-stage

- **GIVEN** a change under changes/archive/
- **WHEN** `specsync set-stage archived-change active` runs
- **THEN** an error is returned
- **AND** .specsync.yaml is not modified

### Scenario: Invalid stage is rejected

- **GIVEN** an invalid stage value: "STAGE NAME" (uppercase, spaces)
- **WHEN** `specsync set-stage my-change "STAGE NAME"` runs
- **THEN** an error is returned
- **AND** no file is written
- **AND** error message names the pattern

### Scenario: Malformed .specsync.yaml blocks mutation

- **GIVEN** a .specsync.yaml with invalid YAML (priority: banana)
- **WHEN** `specsync set-stage my-change active` runs
- **THEN** an error is returned
- **AND** no file is written
- **AND** error message indicates the file must be corrected first

### Scenario: Slug not found

- **GIVEN** no change with slug "nonexistent"
- **WHEN** `specsync set-stage nonexistent active` runs
- **THEN** an error is returned
- **AND** error message suggests checking the slug

## Requirement: set-priority mutates .specsync.yaml

specsync SHALL provide `specsync set-priority` to set or clear a change's priority.

### Scenario: Set priority in .specsync.yaml

- **GIVEN** a change with no .specsync.yaml
- **WHEN** `specsync set-priority my-change 5` runs
- **THEN** .specsync.yaml is created with:
  ```yaml
  version: 1
  priority: 5
  ```

### Scenario: Preserve stage when changing priority

- **GIVEN** a .specsync.yaml:
  ```yaml
  version: 1
  stage: blocked
  priority: 10
  ```
- **WHEN** `specsync set-priority my-change 3` runs
- **THEN** .specsync.yaml becomes:
  ```yaml
  version: 1
  stage: blocked
  priority: 3
  ```
- **AND** stage is unchanged

### Scenario: set-priority unset removes priority

- **GIVEN** a .specsync.yaml with priority=5
- **WHEN** `specsync set-priority my-change unset` runs
- **THEN** priority field is removed

### Scenario: set-priority unset deletes empty .specsync.yaml

- **GIVEN** a .specsync.yaml:
  ```yaml
  version: 1
  priority: 5
  ```
- **WHEN** `specsync set-priority my-change unset` runs
- **THEN** .specsync.yaml file is deleted entirely

### Scenario: Priority out of range

- **GIVEN** priority 0 or 101
- **WHEN** `specsync set-priority my-change 0` (or 101) runs
- **THEN** an error is returned
- **AND** error message states priority must be 1–100
- **AND** no file is written

### Scenario: Priority value boundaries

- **WHEN** `specsync set-priority my-change 1` runs
- **THEN** priority is set to 1 (valid)
- **WHEN** `specsync set-priority my-change 100` runs
- **THEN** priority is set to 100 (valid)

### Scenario: Archived changes accept set-priority

- **GIVEN** a change under changes/archive/
- **WHEN** `specsync set-priority archived-change 5` runs
- **THEN** priority is set (archived changes can be prioritized if re-activated)

## Requirement: Path safety and error handling

specsync SHALL protect against path traversal and reject invalid slugs.

### Scenario: Slug with path traversal is rejected

- **GIVEN** slug "../../../etc/passwd"
- **WHEN** `specsync set-stage "../../../etc/passwd" active` runs
- **THEN** an error is returned
- **AND** no change directory is accessed

### Scenario: Slug with slashes is rejected

- **GIVEN** slug "sub/dir/change"
- **WHEN** `specsync set-stage "sub/dir/change" active` runs
- **THEN** an error is returned

### Scenario: Missing openspec directory

- **GIVEN** no openspec/ directory in current repo
- **WHEN** `specsync changes` runs
- **THEN** an error is returned
- **AND** error message suggests running from a repo with openspec/

## Requirement: Atomic writes

specsync SHALL ensure .specsync.yaml writes are atomic.

### Scenario: Write succeeds or fails cleanly

- **WHEN** `specsync set-stage my-change blocked` runs
- **THEN** .specsync.yaml is written completely or not at all
- **AND** partial file is never left behind
- **AND** existing .specsync.yaml is not corrupted if write fails

## Requirement: CLI stability and discoverability

specsync changes/set-stage/set-priority commands SHALL have stable, self-documenting UX.

### Scenario: Help text is available

- **WHEN** `specsync changes --help` runs
- **THEN** usage, flags, and examples are displayed
- **WHEN** `specsync set-stage --help` runs
- **THEN** usage and examples are displayed
- **WHEN** `specsync set-priority --help` runs
- **THEN** usage and examples are displayed

### Scenario: Missing required arguments

- **WHEN** `specsync set-stage my-change` runs (no stage)
- **THEN** an error is returned
- **AND** help text is shown

### Scenario: Invalid flag

- **WHEN** `specsync changes --invalid-flag` runs
- **THEN** an error is returned
- **AND** help text is shown
