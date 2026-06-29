# commit-source

## ADDED Requirements

### Requirement: Read commits for a revision range
specsync SHALL provide a `CommitSource` capability that returns the parsed
commits reachable in a `[since, until]` revision range. The capability is
optional and detected by type assertion, leaving the minimal provider contract
unchanged.

#### Scenario: Commits since the last tag
- **WHEN** a `CommitSource` is queried with `since` unset and `until` unset
- **THEN** `since` defaults to the most recent reachable tag and `until` defaults to `HEAD`
- **AND** each returned commit is parsed per the conventional-commits capability

#### Scenario: Repository with no tags
- **WHEN** the repository has no tags and `since` is unset
- **THEN** the range starts at the root commit
- **AND** the query succeeds

### Requirement: Git adapter shells out, adding no dependency
The default `CommitSource` SHALL read history by invoking the host `git` CLI
and SHALL NOT introduce a non-standard-library dependency.

#### Scenario: History read via git
- **WHEN** the Git `CommitSource` lists commits
- **THEN** it does so by executing `git log` with a parseable format
- **AND** the package continues to satisfy the stdlib-only invariant

### Requirement: Commit reading is read-only
The `CommitSource` SHALL NOT modify Git state.

#### Scenario: No mutation while reading
- **WHEN** commits are read for any range
- **THEN** no commit, tag, branch, or ref is created, moved, or deleted
