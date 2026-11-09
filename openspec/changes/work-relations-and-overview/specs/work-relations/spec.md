# work-relations

## ADDED Requirements

### Requirement: Build the relations graph from asserted edges
specsync SHALL build a graph of work items whose edges are only: epic parent and child, blocked-by
and blocks, linked changes, slices of one change, request closes issue, and branch to worktree. Each
edge SHALL come from the work provider, git or specsync's own change files, and SHALL NOT be
inferred from names, titles or similarity.

#### Scenario: Request closes issue
- **WHEN** a request closes an issue
- **THEN** the graph has an edge between them from the provider

#### Scenario: Similar titles
- **WHEN** two items have similar titles and no asserted link
- **THEN** no edge is created

### Requirement: Show related work in status
`specsync status` SHALL show the current work together with every item related to it: the epic
chain above it, its siblings, what blocks it and what it blocks. It SHALL mark the current work, show
each item's issue, request, checks and review, show local state where a worktree is known, and end
with the next step.

#### Scenario: Work under an epic
- **WHEN** the current work's issue has an epic parent with three siblings
- **THEN** all four are listed under the epic with the current one marked

#### Scenario: No relations
- **WHEN** the current work has no epic, blockers or links
- **THEN** status shows the work alone

#### Scenario: Not in a worktree
- **WHEN** `status` runs outside any worktree
- **THEN** it shows the ongoing changes of the current repository

### Requirement: List all work with overview
`specsync overview` SHALL list all work in all discovered repositories, grouped by epic and then work
under no epic, with each row's status and relations, and SHALL support `-json`.

#### Scenario: Two repos, one epic
- **WHEN** an epic has items in two repositories
- **THEN** overview shows both under the epic with their requests and states

#### Scenario: Work under no epic
- **WHEN** a branch has a request but no epic
- **THEN** it appears in a separate group for work under no epic

### Requirement: Scope the overview
`overview` SHALL accept `-epic`, `-repo`, `-root`, `-mine`, `-attention`, `-stage` and `-unattached`,
SHALL combine them, and SHALL print which scope was applied.

#### Scenario: Attention in one repo
- **WHEN** `overview -repo api -attention` runs
- **THEN** only rows in that repository that need attention are listed

### Requirement: Discover the workspace
specsync SHALL take roots from `-root`, then `SPECSYNC_ROOTS`, then the parent folder of the current
clone, find main clones to depth 2, list worktrees with `git worktree list --porcelain`, deduplicate
by real path, and map each clone to a tracker repository from its origin URL.

#### Scenario: Worktree outside the roots
- **WHEN** a clone has a worktree in a temporary folder
- **THEN** it is listed and flagged as volatile

#### Scenario: Unmappable clone
- **WHEN** a clone has no origin
- **THEN** a status line names it and the rest are still shown

### Requirement: Show work with no change
specsync SHALL include a row for every discovered branch with a request or worktree even when no change
exists, marked as unlinked.

#### Scenario: Branch with a PR only
- **WHEN** a branch has an open request and no change
- **THEN** it is shown as unlinked with its request and local state

### Requirement: Resolve branches through the request
specsync SHALL resolve a change's branch by naming convention when it matches, and otherwise through
the request that closes its issue, and SHALL NOT require any naming convention.

#### Scenario: Unrelated branch name
- **WHEN** the closing request's branch has a name unrelated to the change
- **THEN** the change's row shows that branch and its worktree

### Requirement: Degrade visibly
When a source is unavailable, specsync SHALL still show what it has, SHALL mark the affected rows or
sections incomplete with the reason, and SHALL NOT present a partial result as complete.

#### Scenario: Provider unreachable
- **WHEN** the provider cannot be reached
- **THEN** local facts are shown and the relations are marked incomplete with the reason

### Requirement: Read-only
specsync SHALL NOT write to any clone, worktree, tracker or stored list of paths when producing
`status` or `overview`.

#### Scenario: Repeated runs
- **WHEN** the same command runs twice with unchanged inputs
- **THEN** the output is identical and nothing outside the existing cache has changed
