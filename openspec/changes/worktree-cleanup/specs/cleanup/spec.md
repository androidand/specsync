# cleanup

## ADDED Requirements

### Requirement: Identify finished work
`specsync cleanup` SHALL list a worktree as a candidate only when its change request is merged or
closed, or its branch is fully merged into the base, and its tree is clean and has no unpushed
commits.

#### Scenario: Merged and clean
- **WHEN** a worktree's request is merged and the tree is clean
- **THEN** it is listed as a candidate with the reason

#### Scenario: Merged but dirty
- **WHEN** a worktree's request is merged but the tree has changes
- **THEN** it is not a candidate and the reason is shown

### Requirement: Dry run by default
`cleanup` SHALL change nothing unless `-apply` is given, and SHALL print each action it would
take with the reason.

#### Scenario: No flag
- **WHEN** `cleanup` runs without `-apply`
- **THEN** no worktree, branch or change is removed

### Requirement: Take configured steps in order
With `-apply`, `cleanup` SHALL remove the worktree, delete the local branch, optionally delete the
remote branch, archive the change when its issue is closed, and run the configured hooks, in that
order, according to configuration.

#### Scenario: Remote branch kept
- **WHEN** `remote_branch` is `keep`
- **THEN** the remote branch is not deleted

#### Scenario: Hook configured
- **WHEN** a cleanup hook is configured
- **THEN** it runs after the other steps for each cleaned worktree

### Requirement: Never remove unsafe work
`cleanup` SHALL NOT remove a worktree that is dirty, has unpushed commits, has an open request, is
the current worktree, or has a live session reported by the session provider.

#### Scenario: Live session
- **WHEN** the provider reports a session in a candidate worktree
- **THEN** it is skipped and the session is named

### Requirement: Report possibly lost work
`cleanup` SHALL list worktrees older than the configured age that are not merged and have
unpushed commits as needing attention, and SHALL NOT remove them.

#### Scenario: Old unpushed work
- **WHEN** a worktree has unpushed commits and no activity for the configured age
- **THEN** it is listed as needing attention and left in place

### Requirement: Cover several repositories
`cleanup` SHALL accept `-root` and apply the same rules to every repository under it.

#### Scenario: Two repos
- **WHEN** two repos under the root each have a finished worktree
- **THEN** both are listed
