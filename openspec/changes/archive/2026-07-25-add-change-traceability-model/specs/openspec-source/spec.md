# openspec-source

## ADDED Requirements

### Requirement: Source OpenSpec data via the openspec CLI
specsync SHALL obtain OpenSpec change metadata, requirement deltas, and
completion status by invoking the `openspec` CLI's machine-readable (JSON)
output, rather than re-parsing spec markdown itself. OpenSpec owns the spec model
and its validation; specsync defers to it.

#### Scenario: Listing changes with status
- **WHEN** the trace engine needs the set of changes and their completion
- **THEN** it reads `openspec list --json` (name, completed/total tasks, status)

#### Scenario: Reading requirement deltas for the release signal
- **WHEN** release impact needs a change's requirement deltas
- **THEN** it reads `openspec show <change> --json --deltas-only`
- **AND** uses the `operation` (`ADDED`/`MODIFIED`/`REMOVED`) per delta

### Requirement: Do not re-implement spec parsing
specsync SHALL NOT maintain its own parser for OpenSpec requirement/delta
structure in the trace features; that responsibility stays with the `openspec`
tool so the two cannot drift.

#### Scenario: Format owned by OpenSpec
- **WHEN** OpenSpec's spec/delta format evolves
- **THEN** specsync's trace features continue to work via the CLI without a parser change on specsync's side

### Requirement: Degrade gracefully without the binary
When the `openspec` CLI is unavailable, specsync SHALL fall back to a minimal
on-disk read (best-effort, without delta operations) and SHALL report that the
authoritative path was unavailable.

#### Scenario: openspec not installed
- **WHEN** `openspec` is not on PATH
- **THEN** trace features still resolve changes from disk where possible
- **AND** requirement-delta signals are reported as unavailable rather than guessed

### Requirement: Treat the openspec JSON as a version-scoped contract
specsync SHALL check the `openspec` version once, require a pinned minimum, and
parse its JSON tolerantly — reading needed fields, ignoring unknown ones, and not
failing on additions — so a Node-side openspec upgrade does not silently break
the trace features.

#### Scenario: openspec older than the minimum
- **WHEN** the installed `openspec` is below the pinned minimum version
- **THEN** specsync reports a clear version error rather than misparsing output

#### Scenario: openspec adds a field
- **WHEN** a newer `openspec` adds a field to the JSON
- **THEN** specsync ignores the unknown field and continues

### Requirement: Query openspec once and cache, not per change
specsync SHALL invoke `openspec` a bounded number of times per run — `list`
once, and `show` at most once per in-scope change — memoizing results, and SHALL
NOT spawn the CLI in a loop over all changes.

#### Scenario: Many changes, bounded spawns
- **WHEN** the trace engine resolves a scope covering many changes
- **THEN** `openspec list` is called once and `show` only for the changes in scope

### Requirement: Reading OpenSpec data is read-only
Sourcing OpenSpec data SHALL NOT modify changes, specs, or status.

#### Scenario: No mutation while reading
- **WHEN** the engine queries `openspec`
- **THEN** no change is archived, edited, or revalidated as a side effect
