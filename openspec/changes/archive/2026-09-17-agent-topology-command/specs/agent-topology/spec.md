## ADDED Requirements

### Requirement: changes project to a single join table

specsync SHALL emit, for each active OpenSpec change across coordinated stores, the
change slug, stage, owning store, tracker issue reference, git branch, git worktree path,
epic parent and linked sibling changes, as one read-only projection.

#### Scenario: a multi-repo change is fully resolved

- **WHEN** two coordinated stores each hold a change linked to a tracker issue, on a
  `feat/<n>-<slug>` branch in its own worktree
- **THEN** the output contains one row per change, each naming its repo, issue URL,
  branch and absolute worktree path

#### Scenario: a change with no worktree is still reported

- **WHEN** a change has an issue but no matching worktree
- **THEN** its row is emitted with a null worktree, and the command exits 0

### Requirement: partial sources narrow the answer, never fail it

Every external source SHALL be optional. When `openspec`, `gh`, a sibling store or git
worktree data is unavailable, specsync SHALL omit only the affected fields, record a
human-readable reason, and still return every row it could build.

#### Scenario: the tracker is unreachable

- **WHEN** `gh` is unauthenticated
- **THEN** rows are emitted with issue id and URL from the local refs cache, issue state
  omitted, and a status line explaining the omission

#### Scenario: nothing could be resolved

- **WHEN** no store yields a single change
- **THEN** the command exits non-zero with the accumulated reasons

### Requirement: the projection is read-only

The command SHALL NOT create, modify or delete changes, issues, branches or worktrees,
and SHALL NOT write cached state as a side effect of being run.

#### Scenario: running it changes nothing

- **WHEN** the command runs against a clean tree
- **THEN** `git status` is unchanged and no tracker write is issued
