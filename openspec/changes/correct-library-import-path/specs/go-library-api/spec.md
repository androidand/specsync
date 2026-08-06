## ADDED Requirements

### Requirement: The documented import path resolves

specsync MUST document exactly one Go import path for embedding it as a library,
and that path MUST be the one the repository actually provides. Documentation,
archived change records, and the package layout MUST agree. A test MUST fail if
they diverge, so the path cannot drift silently the way it did when an archived
change recorded a `pkg/` layout the repository never had.

#### Scenario: A consumer follows the documented path

- **WHEN** a downstream repository imports specsync using the path stated in the
  package documentation
- **THEN** the import resolves against the published module and the package
  compiles

#### Scenario: The package is relocated

- **WHEN** the package is moved to a different directory within the module
- **THEN** the guard test fails until the documented import path is updated to
  match

### Requirement: Archived records describe what shipped

An archived change MUST describe the state the repository is actually in. Where
work was specced and deliberately not done, the record MUST say so rather than
carry a checked task asserting it. Consumers read archived changes as the
reference for how to use what shipped, so a false record propagates into their
code.

#### Scenario: A task claims a layout the repository does not have

- **WHEN** an archived task asserts a file location that does not exist
- **THEN** the record is corrected to state what shipped and why the specced
  alternative was not adopted
