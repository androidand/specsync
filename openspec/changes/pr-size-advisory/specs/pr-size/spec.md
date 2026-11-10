# pr-size

## MODIFIED Requirements

### Requirement: Derive attention flags from a fixed vocabulary
specsync SHALL compute `attention` only from the documented rules, using exactly
these flags: `unpushed`, `uncommitted`, `no-upstream`, `behind-upstream`, `diverged`, `behind-base`,
`base-touches-your-files`, `conflicts-with-base`, `ci-failing`, `changes-requested`, `review-missing`,
`unresolved-comments`, `volatile-path`, `cleanup`, `oversized`, `mixed-scopes`, `no-slice-plan`, and
the positive `ready-to-merge`. Each flag SHALL be a deterministic predicate over the row's facts.
`cleanup` SHALL mean only that the PR is merged and the worktree is clean, and SHALL NOT trigger
any action.

#### Scenario: Same facts, same flags
- **WHEN** two runs read identical git and `gh` facts
- **THEN** both produce identical `attention` lists in identical order

#### Scenario: Merged and clean
- **WHEN** a row's PR is merged and its worktree has no changes
- **THEN** `attention` contains `cleanup`
- **AND** specsync removes nothing

#### Scenario: Size flags are in the vocabulary
- **WHEN** a branch is oversized
- **THEN** `oversized` is a member of the documented vocabulary, not an extension

## ADDED Requirements

### Requirement: Size advice is for unfinished work
The size flags SHALL be added only when the branch has no change request or its request is open.

#### Scenario: Merged request
- **WHEN** a branch's request is merged
- **THEN** none of `oversized`, `mixed-scopes` or `no-slice-plan` is added

### Requirement: Flag oversized branches
specsync SHALL add `oversized` to a row's attention when the branch's size against its
base, computed from git, exceeds the budget on files, changed lines or commits, and
SHALL state which limit was exceeded.

#### Scenario: Over the line budget
- **WHEN** a branch changes more lines than the budget allows
- **THEN** its row carries `oversized` and names the lines limit

#### Scenario: Within budget
- **WHEN** a branch is within every limit
- **THEN** no size flag is added

### Requirement: Flag mixed concerns only from commit scopes
specsync SHALL add `mixed-scopes` when the branch's commits carry more than one distinct
conventional-commit scope, and SHALL list the scopes. It SHALL NOT flag a branch whose
commits carry no scope, and SHALL NOT infer concerns from file paths or content.

#### Scenario: Two scopes
- **WHEN** commits are labelled `feat(billing): ...` and `feat(onboarding): ...`
- **THEN** the row carries `mixed-scopes` listing billing and onboarding

#### Scenario: No scopes used
- **WHEN** no commit carries a scope
- **THEN** `mixed-scopes` is never added

### Requirement: Take the budget from explicit sources
specsync SHALL resolve the budget in this order: the `-budget` flag, the slice's `budget`
line, the `SPECSYNC_BUDGET` environment variable, then documented defaults. It SHALL
accept `-exclude <glob>` to leave matching paths out of the count.

A measure given as 0 SHALL mean no limit for that measure and SHALL override lower sources. A
malformed `-budget` or `SPECSYNC_BUDGET` SHALL be an error; a malformed slice line SHALL be
reported once and ignored for that change. `-exclude` globs SHALL support `**`, and a trailing
`/` SHALL mean everything under that directory.

#### Scenario: Zero removes a limit
- **WHEN** `-budget files=0` is given
- **THEN** no files limit applies, even if a slice line or the environment sets one

#### Scenario: Slice budget wins over the default
- **WHEN** a branch belongs to a slice declaring `budget: files<=5`
- **THEN** that limit is used instead of the default

### Requirement: Suggest a slice plan when none exists
specsync SHALL add `no-slice-plan` to an oversized branch whose change declares no
slices, and SHALL NOT propose any slice boundaries.

#### Scenario: Large change without slices
- **WHEN** an oversized branch's change has no slice headings
- **THEN** the row carries `no-slice-plan`
- **AND** no slices are generated

### Requirement: Verify size on request and optionally fail
`specsync verify -size` SHALL report the size findings for a branch or change request and
exit successfully by default, and SHALL exit non-zero when `-fail-on-oversized` is set and
a limit is exceeded.

#### Scenario: CI gate
- **WHEN** `verify -size -fail-on-oversized` runs on an oversized branch
- **THEN** it exits non-zero and prints the exceeded limits

#### Scenario: Advisory by default
- **WHEN** `verify -size` runs on an oversized branch without the flag
- **THEN** it prints the findings and exits 0
