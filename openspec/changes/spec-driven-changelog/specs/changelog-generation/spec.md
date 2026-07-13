# changelog-generation

## ADDED Requirements

### Requirement: One entry per shipped change

The changelog SHALL contain one entry per OpenSpec change that shipped in the
revision range — a change counts as shipped when the trace links at least one
in-range commit to it — never one entry per commit.

#### Scenario: Multi-commit change collapses to one entry

- **GIVEN** a change `stable-projection-ref-key` linked (via its issue refs) to
  three in-range commits
- **WHEN** `specsync changelog` runs over that range
- **THEN** the output contains exactly one entry for the change, carrying its
  issue reference

### Requirement: Release note sourced from the proposal

An entry's text SHALL be the body of the proposal's `## Release note` section
when present, else the proposal title. The release note is authored at
planning time and synced to the issue like any other proposal section.

#### Scenario: Release note present

- **GIVEN** a shipped change whose proposal.md contains
  `## Release note` followed by "Sync no longer duplicates issues after pull."
- **WHEN** the changelog is built
- **THEN** the entry text is "Sync no longer duplicates issues after pull."

#### Scenario: Release note absent

- **GIVEN** a shipped change with no `## Release note` section
- **WHEN** the changelog is built
- **THEN** the entry text is the proposal's first H1 title

### Requirement: Keep a Changelog categories from existing signals

Entries SHALL be grouped into Added / Changed / Fixed / Removed / Security.
The category derives from the change's OpenSpec requirement deltas
(REMOVED → Removed, ADDED → Added, MODIFIED → Changed) and its linked commit
types (all-fix → Fixed); any breaking signal prefixes the entry with
**Breaking:**. Delta signals outrank commit signals; Removed outranks Added
outranks Changed.

#### Scenario: Added requirements yield an Added entry

- **GIVEN** a shipped change whose deltas are all ADDED
- **WHEN** the changelog is built
- **THEN** the entry appears under "### Added"

#### Scenario: Fix-only change yields a Fixed entry

- **GIVEN** a shipped change with no requirement deltas whose linked commits
  are all `fix:` typed
- **THEN** the entry appears under "### Fixed"

### Requirement: Loose commits are surfaced honestly

In-range commits linked to no change SHALL fall back to their conventional
subject: `feat` under Added, `fix` under Fixed, breaking under Changed with the
**Breaking:** prefix. Non-user-facing conventional types (chore, docs, ci,
build, test, refactor, style, perf without breaking) SHALL be omitted from the
published section, and the count of omitted commits SHALL be reported so the
omission is visible, never silent.

#### Scenario: Loose fix included, loose chore counted

- **GIVEN** an in-range `fix: broken flag parsing` commit and a
  `chore: bump deps` commit, neither linked to any change
- **WHEN** the changelog is built
- **THEN** the fix appears under "### Fixed" and the output notes 1 omitted
  plumbing commit

### Requirement: Idempotent opt-in CHANGELOG.md apply

`-apply` SHALL create CHANGELOG.md (with the Keep a Changelog header) when
absent, prepend the new version section when new, and replace the section
in place when a section for the same version already exists, leaving all other
sections byte-identical. Without `-apply`, the command SHALL write nothing.

#### Scenario: Re-running apply replaces, never duplicates

- **GIVEN** CHANGELOG.md already contains a section for v0.6.0
- **WHEN** `specsync changelog -version 0.6.0 -apply` runs again
- **THEN** CHANGELOG.md still contains exactly one v0.6.0 section

### Requirement: Defers to a changelog-owning release tool

When `DetectReleaseTool` reports a tool that owns "changelog", `-apply` SHALL
refuse with a message naming the tool unless `-force` is given. Read-only
output modes SHALL never be blocked.

#### Scenario: release-please owns the changelog

- **GIVEN** a repo with release-please-config.json
- **WHEN** `specsync changelog -apply` runs
- **THEN** it exits non-zero naming release-please, and `specsync changelog`
  without `-apply` still prints the section
