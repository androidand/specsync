# worktree-path

## ADDED Requirements

### Requirement: Worktree path template
`worktrees.path` SHALL set the path of a new worktree for `start` and `pull -worktree`. It SHALL
expand `{dir}`, `{repo}`, `{slug}`, `{issue}` and `{name}`, where `{name}` is `<issue>-<slug>` with an
issue and `<slug>` without one. When the key is absent the path SHALL be `{dir}/{repo}-{slug}`.

#### Scenario: Nested by repo
- **WHEN** `worktrees.path` is `{dir}/{repo}/{name}` and `start` runs for issue 7 and slug `fix-it`
- **THEN** the worktree is created at `<dir>/<repo>/7-fix-it`

#### Scenario: No key set
- **WHEN** `worktrees.path` is absent
- **THEN** `start` creates the worktree at `<dir>/<repo>-<slug>` as before

### Requirement: One resolver for both commands
`start` and `pull -worktree` SHALL resolve the worktree path, the branch and the `dir` from the same
config, so one change gets the same worktree from either command.

#### Scenario: Pull then start
- **WHEN** `pull -issue 7 -worktree` creates a worktree and `start` then runs for that change
- **THEN** `start` finds the same worktree and creates nothing

### Requirement: Template validation
The config check SHALL report a `worktrees.path` that contains neither `{slug}` nor `{name}`, and
any placeholder it does not know, with the line.

#### Scenario: Shared folder
- **WHEN** `worktrees.path` is `{dir}/{repo}`
- **THEN** the config check reports that every change would share one folder

### Requirement: Orca workspace name
With the Orca launcher and `worktrees.path` set, `start` SHALL pass the last folder of the expanded
path as the Orca workspace name. Without `worktrees.path` it SHALL pass the slug.

#### Scenario: Orca name follows the template
- **WHEN** the launcher is Orca and the path ends in `{name}` for issue 7 and slug `fix-it`
- **THEN** the workspace name is `7-fix-it`
