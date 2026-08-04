# beads

## REMOVED Requirements

### Requirement: Detect bd on PATH

**Reason**: `bd` is installed once per machine, so this rule made every repo look
like a Beads project — auto-selecting Beads in GitHub-tracked repos and creating
phantom items instead of updating existing issues.

**Migration**: Projects that genuinely track work in Beads are detected by their
`.beads/` database (see "Detect a Beads database in the project"). A project with
no `.beads/` selects Beads with an explicit `-provider beads`.

## ADDED Requirements

### Requirement: Detect a Beads database in the project
specsync SHALL auto-select the Beads provider only when a `.beads/` **directory**
is present in the project — at the repo root resolved from `-openspec`, or in the
working directory — and the `bd` binary is on PATH. The presence of `bd` on PATH
SHALL NOT by itself select Beads. Absent a `.beads/` directory, auto-detection
SHALL resolve to github.

#### Scenario: A GitHub project with bd installed stays on github
- **WHEN** `specsync sync` runs with no `-provider` flag in a repo that has no
  `.beads/` directory, on a machine where `bd` is on PATH
- **THEN** the github provider is selected
- **AND** no `bd` command is invoked

#### Scenario: A Beads project is detected at the repo root
- **WHEN** `specsync sync` runs with no `-provider` flag and `.beads/` exists at
  the repo root, with `bd` on PATH
- **THEN** the Beads provider is selected
- **AND** the reason names the `.beads/` directory that was found

#### Scenario: Detection from a subdirectory
- **WHEN** specsync is run from a subdirectory with `-openspec` pointing at the
  project's openspec directory, and `.beads/` exists at that repo root
- **THEN** the Beads provider is selected

#### Scenario: A Beads project without the binary falls back
- **WHEN** `.beads/` exists but `bd` is not on PATH
- **THEN** the github provider is selected
- **AND** specsync does not fail on a `bd` shell-out

#### Scenario: A .beads file is not a database
- **WHEN** `.beads` exists as a regular file rather than a directory
- **THEN** the github provider is selected

### Requirement: Report an auto-detected provider on real runs
specsync SHALL print the chosen provider and the reason on real runs, not only
under `-dry-run`, whenever auto-detection selects a provider other than the
github default — so a redirected sync is visible before its effects are.

#### Scenario: Beads auto-detection is announced
- **WHEN** auto-detection selects Beads for a real `specsync sync`
- **THEN** the output names the provider and why it was chosen

#### Scenario: The github default stays quiet
- **WHEN** auto-detection resolves to github
- **THEN** no provider-detection line is printed
