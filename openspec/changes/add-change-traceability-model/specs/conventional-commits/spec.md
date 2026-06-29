# conventional-commits

## ADDED Requirements

### Requirement: Parse the Conventional Commits header
specsync SHALL parse a commit message header of the form
`type(scope)!: description` into its components, following Conventional Commits
1.0.0, without performing any I/O.

#### Scenario: Full header with scope and breaking marker
- **WHEN** a commit header is `feat(ui)!: split the integration modal`
- **THEN** the parsed type is `feat`
- **AND** the scope is `ui`
- **AND** the description is `split the integration modal`
- **AND** the commit is marked breaking

#### Scenario: Minimal header without scope
- **WHEN** a commit header is `fix: correct off-by-one in slug`
- **THEN** the type is `fix`, the scope is empty, and the commit is not breaking

### Requirement: Detect breaking changes from marker or footer
specsync SHALL mark a commit breaking when the header carries a `!` before the
colon OR when the body contains a `BREAKING CHANGE:` (or `BREAKING-CHANGE:`)
footer.

#### Scenario: Breaking declared in a footer
- **WHEN** a commit header is `refactor: rename Ref key` with a body line `BREAKING CHANGE: cache keys are now namespaced`
- **THEN** the commit is marked breaking
- **AND** the breaking-change description is captured

### Requirement: Extract issue and PR references
specsync SHALL extract issue references (e.g. `#123`, `owner/repo#123`,
`Closes #123`, `Fixes #123`) and pull-request references found in a commit body
or footers.

#### Scenario: Closing footer
- **WHEN** a commit body contains `Closes #42`
- **THEN** `#42` is recorded as an issue reference for that commit

### Requirement: Tolerate non-conventional messages
specsync SHALL NOT error on a commit message that does not conform to the
Conventional Commits grammar; it SHALL mark the commit as not conventional and
preserve the raw message.

#### Scenario: A merge or freeform commit
- **WHEN** a commit message is `Merge branch 'main'`
- **THEN** parsing succeeds with the commit flagged as not conventional
- **AND** the raw message is preserved for reporting
