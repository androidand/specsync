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

### Requirement: The resolved target is reported
specsync SHALL state the concrete resolved repository and which rule produced it, in both dry-run and normal output, before any write occurs.

#### Scenario: Dry run
- **WHEN** a dry run executes
- **THEN** the output names the concrete `owner/name` and the rule that selected it, rather than stating only that the target was auto-detected

#### Scenario: Ambiguity present
- **WHEN** `origin` and `upstream` disagree
- **THEN** the output states which was chosen and how to override it
