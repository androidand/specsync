# spec-verification

## Purpose
Defines how specsync reads a change's behaviour contract and reports whether the
change is ready to archive — OpenSpec's Verify phase, the step between applying
a change and archiving it, without which the contract written during Propose is
never read again.

## ADDED Requirements

### Requirement: Spec deltas are parsed into a change
specsync SHALL parse every `specs/**/spec.md` under a change into capabilities,
requirements and scenarios, recording each requirement's delta operation.

#### Scenario: Requirements and scenarios are counted
- **WHEN** a change carries a delta with two requirements and three scenarios
- **THEN** the loaded change reports two requirements and three scenarios
- **AND** each requirement carries its ADDED or MODIFIED operation

#### Scenario: A change without specs loads cleanly
- **WHEN** a change has no `specs/` directory
- **THEN** it loads without error and reports no deltas

### Requirement: Verification reports an acceptance checklist
specsync SHALL render a change's scenarios as a reviewer-facing checklist, one
line per scenario, qualified by capability and requirement.

#### Scenario: Checklist is rendered for a PR
- **WHEN** `specsync verify -change <slug> -checklist` runs on a contracted change
- **THEN** each scenario appears as an unchecked list item under its requirement
- **AND** the requirement's delta operation is shown

### Requirement: Unverifiable requirements fail verification
specsync SHALL fail verification for a requirement with no scenario, and for a
requirement whose first body line carries neither SHALL nor MUST, because
neither can be checked against an implementation.

#### Scenario: Requirement without a scenario fails
- **WHEN** a delta declares a requirement with no `#### Scenario:` block
- **THEN** verification fails naming that requirement

#### Scenario: Non-normative requirement fails
- **WHEN** a requirement's first body line says "should" rather than SHALL or MUST
- **THEN** verification fails naming that requirement

#### Scenario: Open tasks fail
- **WHEN** a change has unchecked tasks
- **THEN** verification fails reporting how many remain

### Requirement: An uncontracted change warns rather than fails
specsync SHALL report a change with no spec delta as uncontracted and warn, not
fail. Not every change alters observable behaviour, and requiring a contract for
a chore is how a specification becomes ceremony.

#### Scenario: A chore verifies successfully
- **WHEN** a change with no `specs/` delta has all its tasks checked
- **THEN** verification succeeds
- **AND** it warns that nothing will accumulate into `openspec/specs/` on archive

#### Scenario: A placeholder purpose warns
- **WHEN** a capability's Purpose is empty or still says TBD
- **THEN** verification warns naming that capability
