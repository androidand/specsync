# trace

## ADDED Requirements

### Requirement: Expose the resolved trace graph as a command
specsync SHALL provide a `trace` command that prints the resolved trace graph for
a scope, for scripting and debugging beneath `release-plan`. It SHALL accept
`--change <slug>` (a change scope), `--since <ref>` and `--until <ref>` (a
revision-range scope), and `--json`. With no scope flag it SHALL default to the
range "since the last tag" through `HEAD`.

#### Scenario: Trace a single change
- **WHEN** `specsync trace --change add-planning-scan` runs
- **THEN** it prints that change's nodes (work items, PRs, commits) and the links between them, each link carrying its provenance

#### Scenario: Trace a revision range
- **WHEN** `specsync trace --since v0.2.0 --until HEAD` runs
- **THEN** it prints the commits in that range, the changes they link to, and the unlinked commits as reported gaps

### Requirement: Machine-readable trace output
`trace --json` SHALL emit the graph as structured nodes and links, each link
carrying its provenance, so another tool can consume the graph without parsing
the human format.

#### Scenario: JSON consumers read provenance
- **WHEN** `specsync trace --change <slug> --json` runs
- **THEN** the output lists nodes by kind and links with their provenance values

### Requirement: trace is read-only
Resolving and printing a trace SHALL NOT modify any file, Git object, or tracker
item.

#### Scenario: No mutation while tracing
- **WHEN** `trace` runs in any scope
- **THEN** no file, commit, issue, label, or spec is created or changed
