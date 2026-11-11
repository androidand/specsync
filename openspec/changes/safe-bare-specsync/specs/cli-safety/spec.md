# cli-safety

## ADDED Requirements

### Requirement: The empty invocation prints usage
`specsync` with no arguments, and with `-h`, `-help`, `--help` or `help` alone, SHALL print the available commands grouped by purpose and exit 0, without reading from or writing to any tracker.

#### Scenario: Exploring the CLI
- **WHEN** an agent runs `specsync` with no arguments in a repository with OpenSpec changes
- **THEN** it prints the commands and exits 0
- **AND** no tracker command runs

### Requirement: Syncing every change is explicit
A sync that would write to the tracker SHALL name one change with `-change` or every change with `-all`, and SHALL otherwise be refused before contacting the tracker. A dry run needs neither.

#### Scenario: Unscoped sync
- **WHEN** `specsync -repo owner/name` runs without `-change`, `-all` or `-dry-run`
- **THEN** it exits non-zero naming `-change`, `-all` and `-dry-run`
- **AND** no tracker command runs

#### Scenario: Explicit sweep
- **WHEN** `specsync -all` runs
- **THEN** every change is synced as before

#### Scenario: Previewing everything
- **WHEN** `specsync -dry-run` runs without `-change`
- **THEN** it previews every change and writes nothing
