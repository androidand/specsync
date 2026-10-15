# command-discovery

## ADDED Requirements

### Requirement: Make the command and its description one thing
specsync SHALL implement each subcommand as a single type that provides its specification
(name, group, summary, intent phrases, mutation flag, workflow position, safety rules and
examples), a method that defines its flags, and a method that runs it, and SHALL dispatch a
command only through that type. There SHALL be no separate dispatch table, name list or
metadata table.

#### Scenario: Logic without a declaration
- **WHEN** a developer writes command logic without implementing the command type
- **THEN** it cannot be reached from the command line

### Requirement: Register commands automatically and reject incomplete ones
specsync SHALL register each command from its own file and SHALL fail at registration, with a
message naming the command, when its specification has an empty summary, no group or an
undeclared group, no example, a duplicate name, or when any of its flags has no usage text.

#### Scenario: Undocumented command
- **WHEN** a command is added without examples
- **THEN** the binary and the test suite fail at startup naming that command

#### Scenario: New command needs no central edit
- **WHEN** a developer adds a command in its own file
- **THEN** it appears in dispatch and in every derived view without editing any list

### Requirement: Keep examples honest
specsync SHALL fail its tests when an example of a command names a flag that the command's own
flag set does not define.

#### Scenario: Renamed flag
- **WHEN** a flag is renamed but an example still uses the old name
- **THEN** the example test fails and names the command and flag

### Requirement: Keep every visible surface in step with the registered commands
specsync SHALL include a single automated test that compares the commands registered in code
with the commands each visible surface shows, in both directions, covering at least the help
overview, `agent-help`, the README command table, the skill and each derived skill copy, and
dispatch, and SHALL fail naming the surface, the missing commands and the commands shown that
are not registered.

#### Scenario: Command absent from a surface
- **WHEN** a registered command is missing from the README table
- **THEN** the test fails, names the README table and lists the command as missing

#### Scenario: Surface names a command that does not exist
- **WHEN** the skill lists a command that is not registered
- **THEN** the test fails, names the skill and lists the command as not a command

#### Scenario: Registered but not dispatchable
- **WHEN** a registered command does not dispatch
- **THEN** the test fails naming dispatch and the command

#### Scenario: New surface
- **WHEN** a new place that lists commands is added
- **THEN** adding one entry to the test covers it

### Requirement: Forbid a second hand-kept list of commands
The same test SHALL parse the command package's non-test source and SHALL fail when a map,
slice or `switch` outside a command declaration contains three or more registered command
names as string constants, reporting each with file and line.

#### Scenario: Duplicate list reappears
- **WHEN** a developer adds a map of command names in a new file
- **THEN** the test fails naming that file and line and says to derive it from the registry

#### Scenario: Declaration is exempt
- **WHEN** a command's declaration contains its own name
- **THEN** it is not reported

#### Scenario: Today's duplicates
- **WHEN** the test is first run against the code before this change
- **THEN** it reports the dispatch `switch`, the known-subcommand map, the help metadata table
  and the confusions table

### Requirement: Derive documentation from the declaration
specsync SHALL derive the set of known subcommands, unknown-subcommand suggestions,
`agent-help`, the grouped overview, the README command table, the skill command list and
the skill description trigger phrases from the declarations, and SHALL NOT keep a second
hand-written list of commands or flags.

#### Scenario: Add a command
- **WHEN** a developer declares a new command with its summary, group and examples
- **THEN** it appears in `agent-help`, the overview, the README table and the skill after
  regeneration, with no other edit

### Requirement: Describe each flag once
specsync SHALL read each flag's name, type, default and description for `agent-help` from
the command's own flag set, so a flag is described in exactly one place.

#### Scenario: Flag description changed
- **WHEN** a flag's usage text is edited where the flag is defined
- **THEN** `agent-help` shows the new text with no second edit

### Requirement: Fail the build on incomplete or stale documentation
specsync SHALL fail its tests when a declaration has an empty summary, group, example or
flag description, and SHALL fail CI when regenerating the generated documentation produces a
different result from what is committed.

#### Scenario: Missing example
- **WHEN** a declaration has no example
- **THEN** the test suite fails and names the command

#### Scenario: Stale README table
- **WHEN** the README command table differs from the regenerated one
- **THEN** CI fails and says to run the generator

### Requirement: Print a grouped overview as the front door
`specsync help`, `specsync -h` and `specsync` with no arguments and no flags SHALL print the
commands grouped by intent, each group with a starting point, and SHALL NOT mutate any state.
Invocations carrying sync flags SHALL continue to run sync.

#### Scenario: No arguments
- **WHEN** a user runs `specsync` with no arguments
- **THEN** the grouped overview is printed
- **AND** no tracker or file is changed

#### Scenario: Sync flags still sync
- **WHEN** a user runs `specsync -change <slug> -dry-run`
- **THEN** the sync runs in dry-run mode as before

### Requirement: Provide a task-to-command index
specsync SHALL provide `specsync agent-help tasks`, mapping common intents to the command and
flags that serve them, generated from the declarations' intent phrases, and the skill SHALL
contain the same mapping.

#### Scenario: Looking for a status overview
- **WHEN** a user asks `agent-help tasks` how to see the state of their work
- **THEN** the index names `status` for the current work and `overview` for all work

### Requirement: Render every harness's skill from the same declarations
specsync SHALL render the skill or instruction file for each supported agent harness from the
declarations, so that no harness describes a command the binary lacks or omits one it has.

#### Scenario: Two harnesses
- **WHEN** skills are installed for two different harnesses
- **THEN** both list the same commands and the same task index

### Requirement: Report version skew in doctor
`specsync doctor` SHALL compare the binary's declared commands with the commands the
installed skill documents, list those the skill lacks, and warn when the installed skill is
older than the binary.

#### Scenario: Skill lags the binary
- **WHEN** the binary has a command the installed skill does not mention
- **THEN** `doctor` names the command and the remedy `install-skill`

### Requirement: Make unreleased features visible
specsync SHALL provide `specsync changelog -unreleased`, listing changes merged since the
latest release tag.

#### Scenario: Features merged after the last tag
- **WHEN** commits linked to changes exist after the latest tag
- **THEN** the command lists those changes as unreleased
