# Explicit Repo Resolution

## ADDED Requirements

### Requirement: The target repository is resolved explicitly
specsync SHALL resolve a concrete `owner/name` target before issuing any provider command, and SHALL pass it explicitly on every invocation. It SHALL NOT rely on the provider CLI inferring a target from the working directory.

#### Scenario: Command construction
- **WHEN** any provider command is built
- **THEN** it carries an explicit repository argument, and no command is emitted that would let the CLI infer the target

### Requirement: Deterministic resolution order
When no repository is supplied explicitly, specsync SHALL resolve in this order, first match winning: the explicit flag; the repository configured as the CLI's default for this checkout; the `origin` remote.

#### Scenario: Explicit flag supplied
- **WHEN** a repository is passed on the command line
- **THEN** it is used unchanged and no remote is consulted

#### Scenario: Default configured for the checkout
- **WHEN** no flag is supplied and the CLI has a configured default repository for this checkout
- **THEN** that default is used, because it records the user's stated intent

#### Scenario: Plain clone
- **WHEN** no flag is supplied, no default is configured, and only `origin` exists
- **THEN** `origin` is used

### Requirement: Fork parents are never written to implicitly
When the resolved target would be a fork's upstream parent rather than `origin`, specsync SHALL refuse to proceed unless the repository was named explicitly.

#### Scenario: Fork with a diverging upstream
- **WHEN** both `origin` and `upstream` exist, they name different repositories, and no explicit repository was supplied
- **THEN** specsync resolves `origin`, and never the parent

#### Scenario: Parent named deliberately
- **WHEN** the user explicitly names the parent repository
- **THEN** specsync proceeds, because the target was stated rather than inferred

#### Scenario: Write access does not change the outcome
- **WHEN** the user holds write access to the fork parent
- **THEN** the resolution is unchanged and no issue or label is created on the parent, because permission is not intent

### Requirement: The board is never inferred, and never global
specsync SHALL NOT apply a board target that was not declared for the repository being synced. A board SHALL be resolved from an explicit flag or from **repository-local** configuration only. A machine-wide or shell-wide default (an exported environment variable applying to every invocation regardless of directory) SHALL NOT select a board.

#### Scenario: Repository declares no board
- **WHEN** a sync runs in a repository with no declared board and no explicit flag
- **THEN** the issue is created or updated and **no board is touched**, rather than falling back to any default

#### Scenario: A global default is present
- **WHEN** a machine-wide default board is set in the environment and the current repository declares a different board, or declares none
- **THEN** the repository-local declaration wins, or no board is used; the global value never selects the board on its own

#### Scenario: Personal and work boards on one machine
- **WHEN** the user works across a personally-owned repository and an employer-owned repository in the same shell session
- **THEN** each repository syncs only to its own declared board, and neither can reach the other's, because no ambient setting spans them

#### Scenario: Board and repo owner disagree
- **WHEN** the declared board's owner differs from the resolved repository's owner
- **THEN** specsync reports the mismatch and refuses unless the board was named explicitly on the command line

### Requirement: The resolved target is reported
specsync SHALL state the concrete resolved repository and which rule produced it, in both dry-run and normal output, before any write occurs.

#### Scenario: Dry run
- **WHEN** a dry run executes
- **THEN** the output names the concrete `owner/name` and the rule that selected it, rather than stating only that the target was auto-detected

#### Scenario: Ambiguity present
- **WHEN** `origin` and `upstream` disagree
- **THEN** the output states which was chosen and how to override it
