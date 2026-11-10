# status-resume

## ADDED Requirements

### Requirement: Join sessions from an external provider
specsync SHALL accept a provider command through `-sessions-cmd` or
`SPECSYNC_SESSIONS_CMD`, run it once per invocation, read a JSON array from its
standard output, and attach each entry to the status row whose worktree is the
longest path prefix of the entry's `cwd`. specsync SHALL NOT read any agent's
session files itself.

#### Scenario: Session in a worktree subdirectory
- **WHEN** a provider reports a session with a `cwd` inside a row's worktree
- **THEN** that row's `sessions` contains the entry

#### Scenario: No provider configured
- **WHEN** neither the flag nor the environment variable is set
- **THEN** no provider runs and no session fields appear

### Requirement: Surface a resume hint
specsync SHALL show a `resume` hint per row: the provider's `resume` string for the
most recently updated matching session when present, otherwise `cd <worktree>`.
specsync SHALL NOT construct or interpret agent commands.

#### Scenario: After all sessions are lost
- **WHEN** no session is live but the provider reports the last session for a worktree
- **THEN** the row's `resume` is the provider's string for that session

#### Scenario: No session known
- **WHEN** a row has a worktree and no session entry
- **THEN** its `resume` is `cd <worktree>`

### Requirement: Tolerate provider failure
specsync SHALL treat a missing provider, non-zero exit, timeout (default 5 seconds)
or invalid JSON as a status line and SHALL NOT fail the command or change any other
field.

#### Scenario: Provider times out
- **WHEN** the provider does not finish within the timeout
- **THEN** a status line says the session provider timed out
- **AND** the rest of the output is unchanged and the exit code is 0

### Requirement: Take the provider only from the caller's environment
specsync SHALL read the provider command only from the command-line flag or the
caller's environment, and SHALL NOT read or execute it from any file in a repository,
change folder or cache.

#### Scenario: Repository contains a provider setting
- **WHEN** a cloned repository includes a file naming a sessions command
- **THEN** specsync ignores it and runs nothing

### Requirement: Show the last note on a change
specsync SHALL show `lastNote` on rows that have a change, taken from the last
non-empty line of that change's `discoveries.md`, and SHALL omit it when the file is
absent or empty.

#### Scenario: Note recorded earlier
- **WHEN** a change's `discoveries.md` ends with a note
- **THEN** the row shows that line as `lastNote`

### Requirement: Persist nothing about sessions
specsync SHALL NOT write session data to any file, cache or tracker.

#### Scenario: After a run with a provider
- **WHEN** the command finishes
- **THEN** no session information has been stored by specsync
