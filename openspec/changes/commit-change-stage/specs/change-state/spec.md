# change-state

## Purpose
Defines which of a change's state travels with the repository and which stays
local to one machine — the distinction that decides whether a shared plan means
the same thing to everybody reading it.

## ADDED Requirements

### Requirement: Stage and priority are committed
specsync SHALL persist a change's stage and priority in a committed file inside
the change folder, so that a stage set by one person is the stage every other
clone reads.

#### Scenario: A teammate sees the stage
- **WHEN** one person sets a change's stage and commits
- **THEN** a teammate who pulls reads that same stage
- **AND** the value is not in the gitignored projection cache

#### Scenario: Existing local state is migrated, not lost
- **WHEN** a change has a stage only in `.specsync/metadata.json`
- **THEN** specsync reads it
- **AND** the next write records it in the committed file

### Requirement: The projection cache stays local
specsync SHALL keep tracker ids, board bindings and dependency baselines out of
version control, because they identify a projection rather than describe a plan.

#### Scenario: Refs are never committed
- **WHEN** a change is synced and its refs are cached
- **THEN** the cache remains under the gitignored `.specsync/` directory
