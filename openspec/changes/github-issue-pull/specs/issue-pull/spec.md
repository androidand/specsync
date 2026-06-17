# issue-pull

## ADDED Requirements

### Requirement: Materialize a change from an issue
specsync SHALL create a local OpenSpec change folder from an existing tracker
issue identified by its provider id, using a provider that implements the
`IssueReader` capability.

#### Scenario: Pull an issue with a tasks checklist
- **WHEN** `specsync pull -issue 42` runs against a GitHub issue whose body contains a `## Tasks` checklist
- **THEN** `openspec/changes/<slug>/proposal.md` is written from the issue body above the tasks section
- **AND** `openspec/changes/<slug>/tasks.md` is written from the checklist
- **AND** the proposal begins with an H1 derived from the issue title

#### Scenario: Pull an issue without a tasks section
- **WHEN** `specsync pull -issue 42` runs against an issue whose body has no `## Tasks` section
- **THEN** `proposal.md` is written from the whole body
- **AND** no `tasks.md` is written

### Requirement: Preserve the issue↔change link on pull
specsync SHALL bind the new change to the source issue so a later push updates
that same issue rather than creating a duplicate.

#### Scenario: Round-trip pull then push
- **WHEN** a change is created by `specsync pull -issue 42`
- **AND** `specsync -slug <slug>` is run afterwards
- **THEN** the existing issue 42 is updated, not recreated

### Requirement: Resolve the change slug deterministically
specsync SHALL choose the change slug from, in priority order: an explicit
`-slug` flag, the `specsync:change=` marker already present in the issue body,
otherwise a slugified form of the issue title.

#### Scenario: Slug derived from title
- **WHEN** no `-slug` is given and the issue body carries no marker
- **THEN** the slug is the issue title lowercased, with non-alphanumeric runs collapsed to single hyphens

### Requirement: Dry-run pull is side-effect free
A pull invoked with `-dry-run` SHALL print what it would write and make no
filesystem changes.

#### Scenario: Dry-run writes nothing
- **WHEN** `specsync pull -issue 42 -dry-run` runs
- **THEN** no files are created under `openspec/changes/`
- **AND** no ref cache is written
