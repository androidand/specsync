# status-state

## ADDED Requirements

### Requirement: Report pull or merge request state through the work provider
specsync SHALL ask the active work provider for change requests and SHALL attach a
`pr` block to a status row when one exists whose head branch equals the row's
branch, containing number, URL, state, draft flag, review decision, mergeable
state, a checks summary (pass, fail, pending) and size. The data SHALL come from the
provider and SHALL NOT be inferred. A provider without this capability SHALL yield
rows without a `pr` block and no error.

#### Scenario: Open PR with passing checks
- **WHEN** a change's branch has an open PR with nine passing checks
- **THEN** its row has a `pr` block with `state` open and `checks.pass` 9
- **AND** `size` carries the files, additions, deletions and commit count `gh` reports

#### Scenario: No PR yet
- **WHEN** a change's branch has no pull request
- **THEN** its row has no `pr` block and no error is reported

#### Scenario: Provider unavailable
- **WHEN** the provider's tool is missing or unauthenticated
- **THEN** every row omits `pr`
- **AND** a status line says that request state was omitted and why
- **AND** the command exits successfully

#### Scenario: Provider without the capability
- **WHEN** the active provider cannot list change requests
- **THEN** rows carry local facts only and no error is reported

### Requirement: Report local worktree state on each status row
specsync SHALL attach a `local` block to a row that has a worktree, containing the
count of modified and untracked files, the upstream ref, commits ahead and behind
it, the base branch and commits behind the base, the last commit time, and whether
the worktree path is under a system temporary directory, and the size of the branch
against its base (files, additions, deletions, commits) computed from git alone.

#### Scenario: Size without any tracker
- **WHEN** no work provider is configured
- **THEN** each worktree row still reports its size against the base from git

#### Scenario: Unpushed commits
- **WHEN** a worktree's branch is two commits ahead of its upstream
- **THEN** `local.ahead` is 2 and `attention` contains `unpushed`

#### Scenario: Branch never pushed
- **WHEN** a worktree's branch has no upstream
- **THEN** `local.upstream` is empty and `attention` contains `no-upstream`

#### Scenario: Worktree in a volatile directory
- **WHEN** a worktree path is under a system temporary directory
- **THEN** `local.volatilePath` is true and `attention` contains `volatile-path`

### Requirement: Report whether the upstream and the base have moved
specsync SHALL report, for each worktree, commits the branch is behind its own upstream, whether
it has diverged, commits behind the base and when the base last moved, and the files changed on both
the branch and the base since the merge base.

#### Scenario: Teammate pushed to the branch
- **WHEN** the branch's upstream has two commits the local branch lacks
- **THEN** `local.upstreamBehind` is 2 and `attention` contains `behind-upstream`

#### Scenario: Diverged
- **WHEN** the branch has local commits and its upstream has commits the branch lacks
- **THEN** `attention` contains `diverged`

#### Scenario: Base moved over your files
- **WHEN** the base gained commits that touch files the branch also changed
- **THEN** `attention` contains `base-touches-your-files` and the files are listed

### Requirement: Predict conflicts without touching the worktree
specsync SHALL predict whether merging or rebasing onto the base would conflict, listing the paths,
using a computation that does not modify the index or worktree, and SHALL say when the check is
unavailable.

#### Scenario: Would conflict
- **WHEN** the base and the branch change the same lines of one file
- **THEN** `attention` contains `conflicts-with-base` and names the file
- **AND** the worktree is unchanged

#### Scenario: Old git
- **WHEN** git cannot compute the merge without a checkout
- **THEN** the row says the conflict check was unavailable and no flag is added

### Requirement: Suggest a sync strategy
specsync SHALL suggest fast-forward, rebase or merge from a documented table over the freshness facts
and the configured strategy, SHALL list the exact commands, SHALL say when a force push would be
required, and SHALL NOT run any of them.

#### Scenario: Review has started
- **WHEN** the base moved and the request already has reviews
- **THEN** the suggestion is to merge the base and no force push is needed

#### Scenario: Shared branch
- **WHEN** the branch has commits by more than one author
- **THEN** the suggestion is never a force push

#### Scenario: Unreviewed private branch
- **WHEN** the base moved and nobody has reviewed or shared the branch
- **THEN** the suggestion is to rebase, with a force push with lease if already pushed

### Requirement: Report request detail that needs the author
specsync SHALL report failing check names, the count of unresolved review threads when the provider
supplies it, and `ready-to-merge` when the request is approved, checks pass, it is mergeable and not
behind the base where policy requires it.

#### Scenario: Unresolved threads
- **WHEN** a request has three unresolved review threads
- **THEN** `attention` contains `unresolved-comments` and the count is shown

### Requirement: Derive attention flags from a fixed vocabulary
specsync SHALL compute `attention` only from the documented rules, using exactly
these flags: `unpushed`, `uncommitted`, `no-upstream`, `behind-upstream`, `diverged`, `behind-base`,
`base-touches-your-files`, `conflicts-with-base`, `ci-failing`, `changes-requested`, `review-missing`,
`unresolved-comments`, `volatile-path`, `cleanup`, and the positive `ready-to-merge`.
Each flag SHALL be a deterministic predicate over the row's facts. `cleanup` SHALL
mean only that the PR is merged and the worktree is clean, and SHALL NOT trigger
any action.

#### Scenario: Same facts, same flags
- **WHEN** two runs read identical git and `gh` facts
- **THEN** both produce identical `attention` lists in identical order

#### Scenario: Merged and clean
- **WHEN** a row's PR is merged and its worktree has no changes
- **THEN** `attention` contains `cleanup`
- **AND** specsync removes nothing

### Requirement: Show the status of the current work
`specsync status` SHALL resolve the current worktree to its branch and change and report the
change, its issue, request, checks, review, tasks and local state, read-only, using the same
collector as `-json`. Related work is added by `work-relations-and-overview`.

#### Scenario: Inside a worktree
- **WHEN** a user runs `specsync status` inside a worktree whose branch belongs to a change
- **THEN** it shows the change, its issue, request, checks, review, local state and tasks

#### Scenario: Branch without a change
- **WHEN** the current branch has no change
- **THEN** it shows the branch's request and local state and says there is no change

#### Scenario: Attention first
- **WHEN** a change has unpushed commits
- **THEN** the unpushed flag is shown prominently with the count

#### Scenario: Machine output
- **WHEN** an agent runs `specsync status -json`
- **THEN** it receives the same facts, with the `pr`, `local` and `attention` blocks

### Requirement: Rename the unreleased command
specsync SHALL ship the join under the name `status` and SHALL NOT ship a `topology` command
or alias.

#### Scenario: Old name
- **WHEN** a user runs `specsync topology`
- **THEN** it is reported as an unknown subcommand with a hint to use `status`

### Requirement: Allow a local-only run
specsync SHALL accept `-state=off`, which omits all `gh` calls and returns only
facts available from git, so the command is instant and works offline.

#### Scenario: Offline
- **WHEN** the user runs `specsync status -state=off`
- **THEN** no network or `gh` call is made
- **AND** rows still show local state and attention flags that need no PR data
