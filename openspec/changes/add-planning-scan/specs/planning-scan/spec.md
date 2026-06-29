# planning-scan

## ADDED Requirements

### Requirement: Report what already exists for an area
specsync SHALL provide a `scan` command that, given an area (path globs and/or a
topic string), reports the planning-relevant slice of the trace graph: in-flight
OpenSpec changes related to the area with their status, open issues in the area
with no linked change, and recent commits, PRs, and releases that touched the
same files.

#### Scenario: Scan by path and topic
- **WHEN** `specsync scan src/integrations/ "integration modal"` runs
- **THEN** it lists in-flight changes that touch those paths or match the topic, each with its OpenSpec status
- **AND** it lists open issues in the area that link to no change
- **AND** it lists recent commits/PRs/releases touching those paths

#### Scenario: Topic with nothing yet
- **WHEN** `specsync scan "a brand-new idea"` matches no existing artifact
- **THEN** the scan reports that nothing exists here yet
- **AND** does not invent a relationship

### Requirement: Deterministic and read-only, no inference
`scan` SHALL be deterministic and read-only, sourcing only from `openspec`,
`git`, and `gh`, and SHALL NOT invoke an LLM, build a semantic code graph, or
modify any state.

#### Scenario: Stable repeated runs
- **WHEN** `scan` is run twice on an unchanged repository
- **THEN** it produces identical output

#### Scenario: No mutation
- **WHEN** `scan` runs
- **THEN** no file, Git object, tracker item, or spec is changed

### Requirement: Provide machine-readable output for planning agents
`scan` SHALL offer `--json` output structured for a planning agent, with each
item carrying its identifier and link provenance.

#### Scenario: Agent reads the area before planning
- **WHEN** `specsync scan <area> --json` runs
- **THEN** the output lists related changes (with status), issues, commits, and PRs with provenance
- **AND** an agent can cite them when authoring a proposal

### Requirement: Degrade gracefully when a source is unavailable
When a source CLI is unavailable, `scan` SHALL report what it could not reach
rather than silently narrowing the result.

#### Scenario: No tracker access
- **WHEN** `gh` is unavailable
- **THEN** `scan` still reports changes and commits
- **AND** notes that issues and PRs could not be read
