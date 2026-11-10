# cleanup

## ADDED Requirements

### Requirement: Identify finished work
`specsync cleanup` SHALL list a worktree as a candidate only when its work is provably in the base
and its tree is clean. Work is provably in the base when the change request is merged and the branch
head equals the head commit the request merged with, or when the branch head is an ancestor of the
base. A request closed without merging, or a branch with commits pushed after the merge, SHALL NOT
make a worktree a candidate.

#### Scenario: Merged and clean
- **WHEN** a worktree's request is merged and the tree is clean
- **THEN** it is listed as a candidate with the reason

#### Scenario: Closed without merging
- **WHEN** a worktree's request was closed unmerged
- **THEN** it is not a candidate and is reported as needing attention if it has unpushed commits

#### Scenario: Commits after the merge
- **WHEN** a request merged and the branch has a later local commit
- **THEN** it is not a candidate and the extra commit is named

#### Scenario: Upstream deleted by the host
- **WHEN** the upstream branch is gone and the branch head is neither the merged head nor in the base
- **THEN** its commits count as unpushed and the worktree is not a candidate

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
With `-apply`, `cleanup` SHALL run the configured hooks in the worktree, remove the worktree, delete
the local branch and optionally delete the remote branch, in that order, according to configuration.
It SHALL NOT archive a change; when `cleanup.archive` is true and the issue is closed it SHALL report
`specsync archive -change <slug>` as a follow-up.

#### Scenario: Remote branch kept
- **WHEN** `remote_branch` is `keep`
- **THEN** the remote branch is not deleted

#### Scenario: Hook configured
- **WHEN** a cleanup hook is configured and the config is trusted
- **THEN** it runs first, in the worktree, for each cleaned worktree

#### Scenario: Untrusted config
- **WHEN** a hook is configured in a config the user has not trusted with `specsync trust`
- **THEN** no hook runs, the hook commands are printed and the worktree is skipped, not removed

#### Scenario: Several repos
- **WHEN** `-root` covers repos with different trust states
- **THEN** each repo's hooks run only if that repo's config is trusted

### Requirement: Never remove unsafe work
`cleanup` SHALL NOT remove a worktree that is dirty, has unpushed commits, has an open request, is
the current worktree, or has a live session reported by the session provider.

#### Scenario: Live session
- **WHEN** the provider reports a session in a candidate worktree
- **THEN** it is skipped and the session is named

### Requirement: Report possibly lost work
`cleanup` SHALL list worktrees older than the configured age whose work is not provably in the base
and that have unpushed commits, and SHALL NOT remove them.

#### Scenario: Old unpushed work
- **WHEN** a worktree has unpushed commits and no activity for the configured age
- **THEN** it is listed as needing attention and left in place

### Requirement: Cover several repositories
`cleanup` SHALL accept `-root` and apply the same rules to every repository under it.

#### Scenario: Two repos
- **WHEN** two repos under the root each have a finished worktree
- **THEN** both are listed

### Requirement: Protect branches and ignored files
`cleanup` SHALL NOT remove or delete a protected branch (main, master, develop, trunk, staging, production, prod, gh-pages, release/*, hotfix/*, the `cleanup.protect` globs, the base or default branch, or the branch checked out in the main checkout) or a branch with commits from several authors, and SHALL NOT remove a worktree that holds ignored files other than common build output (node_modules, dist, build, target, .next, vendor, __pycache__) unless `-include-ignored` is given. Both SHALL be checked again immediately before removal.

#### Scenario: Ignored file
- **WHEN** a finished worktree holds an ignored `debug.log`
- **THEN** it is skipped as "has ignored files: debug.log"

### Requirement: Order and fresh checks
For each candidate `cleanup` SHALL re-verify it from fresh facts, then run hooks in the worktree (only when trusted), remove the worktree, delete the local branch only while it still points at the verified head, and delete the remote branch only when the upstream has the same remote and name and its tip, read at that moment, is the head being removed or the merged head. A failed hook SHALL leave the worktree and branch in place. Unreadable git status or ignored-file checks SHALL make the worktree not a candidate. A request merged into a branch other than the base SHALL NOT count as proof.

#### Scenario: Remote moved on
- **WHEN** someone pushed to the remote branch after the merge
- **THEN** the remote branch is kept and the reason is reported

### Requirement: Say when sessions were not checked
When no session provider is configured, it failed, or `-state off` is used, `cleanup` SHALL say that live sessions were not checked, and `-apply` SHALL refuse unless `-assume-no-sessions` is given.

### Requirement: History is not proof
A branch whose head is in the base SHALL be a candidate only if its reflog shows commits made on it and no reset; a reset branch needs a merged request with this head. Deleting a branch SHALL record how to restore it and remove its branch configuration. Hooks SHALL run in the main checkout with the worktree path in `SPECSYNC_WORKTREE`, limited to 60 seconds with capped output. A config with problems SHALL make items skip with the problem shown. A dry run SHALL write no git objects and no request cache. The remote branch SHALL be kept while an open request targets it or when its tip is not the head being removed.

