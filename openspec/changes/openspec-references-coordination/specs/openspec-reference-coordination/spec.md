# openspec-reference-coordination

## ADDED Requirements

### Requirement: Read OpenSpec coordination instead of duplicating it
specsync SHALL discover sibling repos from OpenSpec's own coordination data — the
`references:` resolved by `openspec context --json` and the folder sets from
`openspec workset list --json` — and SHALL NOT maintain its own registry of repos
or local paths.

#### Scenario: Siblings come from OpenSpec, not a specsync file
- **WHEN** the repo's `openspec/config.yaml` declares `references: [backend]`
- **THEN** specsync learns the backend store and its local path from `openspec context --json`
- **AND** specsync writes no path registry of its own

### Requirement: Surface referenced siblings in planning output
specsync SHALL report, for the current repo, each referenced sibling repo, its
resolved local folder, and the sibling's related changes/issues, so an agent in
one worktree can locate and compare with another.

#### Scenario: An agent in the frontend worktree sees the backend
- **WHEN** `scan`/`relate` runs in a repo that references the backend store
- **THEN** the output lists the backend repo, its local folder, and its related changes/issues
- **AND** an agent can open or compare that folder

### Requirement: Suggest, never auto-create, the tracker edge
When a reference implies a dependency, specsync SHALL suggest a `## Blocked by`
entry for confirmation and SHALL NOT write a GitHub dependency directly from a
repo-level reference. Projecting confirmed edges remains the job of the explicit
typed-link sync.

#### Scenario: Reference suggests a dependency
- **WHEN** the frontend repo references the backend store and a related backend issue exists
- **THEN** specsync suggests adding `## Blocked by` → the backend issue
- **AND** it does not create the GitHub dependency until the entry is confirmed in `links.md`

### Requirement: Degrade cleanly without OpenSpec coordination data
specsync SHALL function when no `references:`/`workset` exist or the `openspec`
binary is older or absent, simply reporting no extra siblings rather than failing.

#### Scenario: No references configured
- **WHEN** the repo declares no `references:` and has no worksets
- **THEN** planning output omits the siblings section
- **AND** no error is raised
