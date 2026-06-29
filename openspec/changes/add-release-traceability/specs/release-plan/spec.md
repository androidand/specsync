# release-plan

## ADDED Requirements

### Requirement: Report what shipped and what is loose
specsync SHALL provide a `release-plan` report for a revision range that lists
the OpenSpec changes included, the issues and pull requests linked, the
contributing commits, the work that links to no change (loose ends), the changes
whose tasks are all complete (archive candidates), and an advisory bump.

#### Scenario: A follow-up report over the default range
- **WHEN** `specsync release-plan` runs with the range defaulting to "since the last tag"
- **THEN** it lists the changes, issues, PRs, and commits in the range
- **AND** it lists work that links to no OpenSpec change as loose ends
- **AND** it lists changes whose tasks are all done as archive candidates
- **AND** it prints the advisory bump with reasons

### Requirement: Surface loose ends from the trace graph
specsync SHALL derive loose ends from the trace graph's reported gaps rather than
fabricating links, so that a commit or PR realizing no change is reported, not
guessed.

#### Scenario: A feature PR without a spec
- **WHEN** a `feat` commit or its PR links to no OpenSpec change
- **THEN** the report names it under loose ends

### Requirement: Read-only by default
`release-plan` and `trace` SHALL be read-only unless `--apply` is given. Without
it, no file, Git object, tracker item, or release is modified.

#### Scenario: Plain run mutates nothing
- **WHEN** `specsync release-plan` runs without `--apply`
- **THEN** no spec is archived, no changelog is written, and no Git or tracker state changes

### Requirement: Gate spec actions behind an explicit flag
specsync SHALL perform suggested spec actions (archiving a completed change) only
when `--apply` is given, and SHALL still never bump versions, write tags,
publish, or edit tracker issues.

#### Scenario: Applying spec actions only
- **WHEN** `specsync release-plan --apply` runs and a change's tasks are all complete
- **THEN** that change is archived locally
- **AND** no Git tag, version bump, release, or issue edit is made

### Requirement: Defer release mechanics to the detected tool
The report SHALL state the detected release path and that specsync defers to it
for bumping, tagging, changelog, and publishing; the printed bump is advisory.

#### Scenario: Deferring to the project's tooling
- **WHEN** a release tool (or a custom tag-based flow) is detected
- **THEN** the report names it and the responsibilities it owns
- **AND** marks specsync's bump as advisory only

### Requirement: Per-package report in a monorepo
When packages are configured, specsync SHALL compute and print a separate
advisory bump per package based on the artifacts whose paths match it.

#### Scenario: Two packages, different bumps
- **WHEN** commits touch package A with a `feat` and package B with only `chore`
- **THEN** package A is advised `minor` and package B `none`
