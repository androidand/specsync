## ADDED Requirements

### Requirement: Detect the `openspec` binary
`specsync doctor` and `specsync doctor claude` SHALL check whether the
`openspec` binary is reachable on `$PATH` and SHALL report its resolved path
and reported version when found.

#### Scenario: openspec binary present
- **WHEN** `openspec` is on `$PATH`
- **THEN** `specsync doctor claude` reports the dependency as found
- **AND** includes the resolved binary path and, when parseable, the version
  reported by `openspec --version`

#### Scenario: openspec version unparseable
- **WHEN** `openspec` is on `$PATH` but `openspec --version` fails or its
  output cannot be parsed
- **THEN** `specsync doctor claude` still reports the dependency as found
- **AND** reports the version as unknown rather than failing the diagnostic

### Requirement: Warn when the `openspec` binary is missing
`specsync doctor` SHALL surface a missing `openspec` binary as a warning-level
issue with a recommendation, rather than only failing later inside sync/pull/
scan commands.

#### Scenario: openspec binary missing
- **WHEN** `openspec` is not found on `$PATH`
- **THEN** `specsync doctor claude` reports `status: "warning"` (or worse)
- **AND** includes a recommendation to run `openspec init` (or otherwise
  install the `openspec` CLI)

### Requirement: Report the dependency check in `doctor install`
`specsync doctor install` SHALL list the `openspec` binary dependency
alongside the existing per-agent skill installation locations.

#### Scenario: Combined view
- **WHEN** a user runs `specsync doctor install`
- **THEN** the output includes an entry for the `openspec` binary (found/not
  found, path, version) in addition to the existing skill-location entries

### Requirement: JSON output includes the dependency check
`specsync doctor claude --json` and `specsync doctor install --json` SHALL
include the `openspec` binary check as additive fields, without removing or
renaming existing fields.

#### Scenario: Machine-readable dependency status
- **WHEN** `specsync doctor claude --json` is run
- **THEN** the JSON output includes a dependency-check object for `openspec`
  with `found`, `path`, and `version` fields
- **AND** all previously existing fields remain present and unchanged in
  shape

#### Scenario: `doctor install --json` starts emitting JSON at all
- **WHEN** `specsync doctor install --json` is run
- **THEN** the command emits structured JSON (today it silently ignores
  `--json` and always prints prose) including the dependency-check fields
