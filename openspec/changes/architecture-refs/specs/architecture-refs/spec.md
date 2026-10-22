# architecture-refs

## ADDED Requirements

### Requirement: Architecture refs are parsed and kept apart
specsync SHALL read entries of the form `<scheme>:<path>` under `## Architecture`
in `links.md` as architecture refs, and SHALL NOT treat them as related links or
dependencies.

#### Scenario: Architecture entries do not become related issues
- **WHEN** `links.md` has `## Architecture` with `- archsync:owner/repo/logical.service.fraud`
- **THEN** the change has one architecture ref with id `logical.service.fraud`
- **AND** the issue's `## Related` section does not list it

### Requirement: Architecture refs are asserted graph edges
specsync SHALL add an `arch` node and a `spec-arch` edge with source `links.md` for
each architecture ref, and SHALL NOT create such edges from any other source.

#### Scenario: Relate by element
- **WHEN** two changes record `archsync:owner/repo/logical.service.fraud` and a third does not
- **THEN** `specsync relate -arch archsync:owner/repo/logical.service.fraud` lists exactly the two changes

### Requirement: Issues show their architecture scope
specsync SHALL render an `## Architecture` section in a synced issue body listing
the element ids of the change's architecture refs, and SHALL render none when the
change has no architecture refs.

#### Scenario: Change with architecture refs
- **WHEN** a change with two architecture refs is synced
- **THEN** the issue body contains an `## Architecture` section with both element ids
