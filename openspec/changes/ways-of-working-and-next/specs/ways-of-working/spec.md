# ways-of-working

## ADDED Requirements

### Requirement: Read the flow from repository config
specsync SHALL read an optional `flow` section from `openspec/specsync.yml` naming a preset or
listing steps with their order, preconditions and gates, SHALL use the default `pr-review`
preset when absent, and SHALL accept the presets `minimal`, `pr-review` and `strict`.

#### Scenario: No config
- **WHEN** the file has no `flow` section
- **THEN** the default preset is used

#### Scenario: Unknown preset
- **WHEN** the config names a preset that does not exist
- **THEN** `doctor` reports it and names the valid presets

### Requirement: Say what to do next
`specsync next` SHALL read the current work's facts from the status collector, apply the flow,
and print the next step with the exact command, and SHALL NOT run it or change any state. With
`-json` it SHALL emit the step, the command and the reason.

#### Scenario: Gates not run
- **WHEN** all tasks are done and no gate stamp exists for the current commit
- **THEN** `next` names the gates step and prints the command to run

#### Scenario: Work is merged
- **WHEN** the work's request is merged and its worktree is clean
- **THEN** `next` names the cleanup step

#### Scenario: Nothing in progress
- **WHEN** the current folder has no change or branch of interest
- **THEN** `next` suggests starting or creating work

### Requirement: Show the flow and position
`specsync flow` SHALL print the configured steps in order and mark the step the current work is
at.

#### Scenario: Mid-flow
- **WHEN** a change has a PR awaiting review
- **THEN** `flow` marks the review step as current and earlier steps as done

### Requirement: Define gates in config
specsync SHALL read gates as ordered named commands with an optional flag, and SHALL treat a gate
as part of the flow only when it is not optional or when requested.

#### Scenario: Optional gate
- **WHEN** a gate is marked optional
- **THEN** it runs only when requested by name

### Requirement: Never orchestrate
specsync SHALL NOT launch agents, run in the background, or execute a step because `next` or
`flow` was invoked.

#### Scenario: Read-only
- **WHEN** `next` or `flow` runs
- **THEN** no file, branch, tracker item or process is created or changed

### Requirement: Validate configuration
`specsync doctor` SHALL validate `openspec/specsync.yml` against a documented schema and report
unknown keys, wrong types and missing gate commands with the line.

#### Scenario: Typo in a key
- **WHEN** the config has `gate:` instead of `gates:`
- **THEN** `doctor` reports the unknown key and suggests the right one

### Requirement: Run repository commands only when trusted
specsync SHALL run gates, hooks and any other command defined in `openspec/specsync.yml` only when the
current content of that file has been trusted by the user through `specsync trust`, SHALL record trust
per user and per repository as a hash of the config, and SHALL refuse and print the commands when
trust is missing or the config has changed.

#### Scenario: Freshly cloned repo
- **WHEN** a user runs `specsync gates` in a repo whose config was never trusted
- **THEN** nothing runs and the commands that would run are printed with how to trust them

#### Scenario: Config changed
- **WHEN** the config changes after it was trusted
- **THEN** trust is required again before any command runs

### Requirement: Read the sync strategy from config
specsync SHALL read `sync.strategy` (rebase or merge), `sync.after_review` and `sync.force` from the
config, SHALL apply the documented defaults when absent, and SHALL use them for sync suggestions and
for `update`.

#### Scenario: Project prefers merge
- **WHEN** the config sets the strategy to merge
- **THEN** suggestions and `update` merge the base instead of rebasing
