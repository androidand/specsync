# flow-verbs

## ADDED Requirements

### Requirement: Start work in a worktree from config
`specsync start <ref>` SHALL resolve the reference to a change and issue, reuse an existing
worktree for it when one exists, otherwise create a worktree and branch using the configured
directory and branch pattern, run the configured start hooks, and record the link between branch
and change. It SHALL refuse when a live session is reported in that worktree.

#### Scenario: Existing worktree
- **WHEN** a worktree already exists for the change
- **THEN** `start` reports it and creates nothing

#### Scenario: New work
- **WHEN** no worktree exists
- **THEN** one is created with a branch named from the pattern
- **AND** the start hooks run in it

#### Scenario: Live session present
- **WHEN** the session provider reports a session in the worktree
- **THEN** `start` refuses and names the session

### Requirement: Create a change from a template
`specsync new <title>` SHALL create a change from a short template, search existing changes and
issues for duplicates first and show them, and SHALL create nothing in the tracker until the
issue text has been shown and confirmed.

#### Scenario: Probable duplicate
- **WHEN** an open issue matches the title's key words
- **THEN** it is shown before anything is created

### Requirement: Save safely
`specsync save` SHALL commit, push and sync the spec, SHALL refuse files that look like secrets
and trailers listed as forbidden in config, and SHALL refuse to push a branch that has an open
request unless told to.

#### Scenario: Forbidden trailer
- **WHEN** the commit message contains a trailer the config forbids
- **THEN** `save` refuses and names the trailer

### Requirement: Run gates and stamp the result
`specsync gates` SHALL run the configured gates in order, stop reporting at the first failure
unless told to continue, and write a stamp tied to the current commit.

#### Scenario: Stamp goes stale
- **WHEN** a new commit is made after the stamp
- **THEN** the stamp is no longer valid for `pr prep`

### Requirement: Prepare and open a pull request
`specsync pr prep` SHALL check that tasks are done, the branch is current with its base, the gate
stamp is valid for the head commit, and the size is within budget, and SHALL print what fails.
`specsync pr open` SHALL create the request with a generated body only after the configured
confirmations are given.

#### Scenario: Gates not run
- **WHEN** no valid stamp exists
- **THEN** `pr prep` fails and names the gates step

#### Scenario: Unconfirmed
- **WHEN** a required confirmation is missing
- **THEN** `pr open` refuses and lists the missing confirmations

### Requirement: Park work
`specsync park` SHALL save, set the change stage and append a note so another session can resume.

#### Scenario: Stopping for the day
- **WHEN** `park -m "..."` runs
- **THEN** work is saved, the stage is set and a note is recorded

### Requirement: Keep local conventions in configuration
specsync SHALL read worktree location, branch pattern, start hooks, gates, forbidden trailers and
required confirmations from `openspec/specsync.yml` and SHALL contain no project-specific
conventions in code.

#### Scenario: Different repo, different pattern
- **WHEN** two repos configure different branch patterns
- **THEN** each creates branches by its own pattern

### Requirement: Bring a branch up to date safely
`specsync update` SHALL print the plan from the configured strategy and the sync suggestion, SHALL apply
it only with `-apply`, SHALL refuse on a dirty tree, SHALL never force-push a shared branch, and SHALL
push, with a lease, only when `-push` is given.

#### Scenario: Plan only
- **WHEN** `update` runs without `-apply`
- **THEN** it prints the steps and changes nothing

#### Scenario: Conflicts predicted
- **WHEN** a conflict is predicted
- **THEN** the plan lists the files and `-apply` still stops at the conflict for the user to resolve

#### Scenario: Shared branch
- **WHEN** the branch has commits by more than one author
- **THEN** `update` merges and never rewrites history
