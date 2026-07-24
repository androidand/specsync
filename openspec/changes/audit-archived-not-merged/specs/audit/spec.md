# audit

## ADDED Requirements

### Requirement: Read PR state from GitHub
specsync SHALL provide methods on `GitHubProvider` to query open and recently
merged PRs via `gh pr list`, returning structured `PRState` values with number,
URL, title, branch name, body, and merge status.

#### Scenario: List open PRs
- **WHEN** `ListOpenPRs` is called on a GitHub provider
- **THEN** it returns PRs in the open state with their number, URL, title, branch name, and body
- **AND** the provider uses `gh pr list --state open` under the hood

#### Scenario: List merged PRs
- **WHEN** `ListRecentMergedPRs` is called on a GitHub provider
- **THEN** it returns recently merged PRs with their number, URL, title, branch name, and body
- **AND** the provider uses `gh pr list --state merged` under the hood

### Requirement: Cross-reference archived changes against PR state
specsync SHALL provide an `audit` command that loads archived OpenSpec changes,
queries GitHub for open and merged PRs, and classifies each archived change as
unmerged (open PR exists), shipped (merged PR exists), or orphaned (no PR at all).

#### Scenario: Archived change with open PR
- **WHEN** an archived change has an open PR whose branch name or title contains the change slug
- **THEN** `specsync audit` reports it as "unmerged" with the PR link
- **AND** the output includes the change slug, PR number, and PR URL

#### Scenario: Archived change with merged PR
- **WHEN** an archived change has a merged PR whose branch name or title contains the change slug
- **THEN** `specsync audit` reports it as "shipped" with the PR link

#### Scenario: Archived change with no PR
- **WHEN** an archived change has no open or recently merged PR matching its slug
- **THEN** `specsync audit` reports it as "orphaned"

#### Scenario: Fail-fast mode
- **WHEN** `specsync audit -fail-on-unmerged` is run and unmerged changes exist
- **THEN** it exits with a non-zero status code
- **AND** the error message lists the unmerged changes

### Requirement: Match PRs to changes by slug
specsync SHALL match a PR to a change when the change slug appears in the PR's
branch name prefix, PR title, or specsync marker comment in the PR body. A PR
matching the marker comment takes priority over other matching strategies.

#### Scenario: Match by branch name
- **WHEN** a PR's branch name is `audit-archived-not-merged` or `skein/audit-archived-not-merged`
- **THEN** it matches the change slug `audit-archived-not-merged`

#### Scenario: Match by specsync marker
- **WHEN** a PR's body contains the comment `<!-- specsync:change=audit-archived-not-merged -->`
- **THEN** it matches the change slug `audit-archived-not-merged`

### Requirement: Provide machine-readable audit output
specsync SHALL offer `specsync audit -json` that outputs the audit findings as
structured JSON, suitable for CI pipelines and programmatic consumption.

#### Scenario: JSON output for CI
- **WHEN** `specsync audit -json` runs
- **THEN** the output is valid JSON with a `findings` array
- **AND** each finding has `slug`, `status`, and `pr` fields

### Requirement: Add shipped stage to canonical lifecycle
specsync SHALL add `shipped` as a new canonical stage between `archived` and end
of lifecycle. The shipped stage indicates the change is archived AND the
corresponding PR has merged to master.

#### Scenario: Shipped stage is valid
- **WHEN** `ValidateStage("shipped")` is called
- **THEN** it returns nil (valid)

#### Scenario: Shipped appears after archived in canonical order
- **WHEN** `CanonicalStageOrder()` is called
- **THEN** `shipped` appears after `archived` in the returned slice

### Requirement: Opt-in mark-shipped metadata write
specsync SHALL provide `-mark-shipped` flag on `audit` that writes `stage:
shipped` to the change's `.specsync/metadata.json` when a merged PR is
confirmed. This is opt-in because writing metadata is a mutation.

#### Scenario: Mark shipped for merged PR
- **WHEN** `specsync audit -mark-shipped` is run and an archived change has a merged PR
- **THEN** the change's `.specsync/metadata.json` is written with `stage: shipped`
- **AND** a subsequent `specsync audit` run reports the change as shipped (not re-queried)
