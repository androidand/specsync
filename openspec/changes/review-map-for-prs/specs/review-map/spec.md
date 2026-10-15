# review-map

## ADDED Requirements

### Requirement: Produce a review checklist for a branch or range
`specsync review-map` SHALL accept a branch, a revision range or a change request and
print, in commit order, each commit with its files, lines added and removed, and a
category, followed by totals per category. It SHALL use only git, and the provider only
to resolve a change request given by number or URL.

#### Scenario: Branch of five commits
- **WHEN** a branch has five commits against its base
- **THEN** the map lists the five in order, each with its files and sizes
- **AND** the totals per category are shown

### Requirement: Mark judgement areas only from declared focus
specsync SHALL mark a file as needing human judgement only when it matches a glob in the
change's `## Review focus` section, showing the declared reason, and SHALL mark nothing
when the section is absent.

#### Scenario: Declared focus
- **WHEN** the section lists `src/auth/**` with a reason
- **THEN** changed files under it appear first under needs judgement with that reason

#### Scenario: No focus section
- **WHEN** the change has no review focus section
- **THEN** the map orders and categorises files and marks none as needing judgement

### Requirement: Categorise with language-neutral rules
specsync SHALL assign each file one category from source, test, docs, config, generated
or other using documented path patterns, and SHALL NOT inspect file contents.

#### Scenario: Lockfile and test
- **WHEN** a change touches a dependency lockfile and a file in a test directory
- **THEN** they are categorised as generated and test respectively

### Requirement: Link commits to tasks only by explicit reference
specsync SHALL show a commit under a task only when its message names that task's id, and
SHALL show all other commits as unlinked, never matching by similarity.

#### Scenario: Commit names a task
- **WHEN** a commit message contains `task 2.1`
- **THEN** it is listed under task 2.1

### Requirement: Output for people and agents
specsync SHALL print Markdown with checkboxes by default and structured JSON with
`-json`.

#### Scenario: Structured output
- **WHEN** `-json` is given
- **THEN** the same content is emitted as JSON with the same ordering

### Requirement: Prove a rewritten branch has the same content
`specsync review-map -equivalent <old> <new>` SHALL compare the trees of the two
revisions and report that they are identical or list the paths that differ, and SHALL
state that it compares content, not history.

#### Scenario: History regrouped, content unchanged
- **WHEN** a branch was rewritten into fewer commits with identical results
- **THEN** the command reports the trees identical

#### Scenario: Content differs
- **WHEN** the rewritten branch differs in two files
- **THEN** the command lists those two paths and exits non-zero
