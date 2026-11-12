# gates-if-stale

## ADDED Requirements

### Requirement: Skip fresh gates
`specsync gates -if-stale` SHALL run no gate and exit 0 when the gate stamp names HEAD and the
working tree is clean. Otherwise it SHALL run the gates exactly as `specsync gates` does.

#### Scenario: Stamp is fresh
- **WHEN** the gates passed on HEAD and the tree is clean
- **THEN** `-if-stale` runs nothing and exits 0

#### Scenario: New commit
- **WHEN** a commit was made after the gates passed
- **THEN** `-if-stale` runs the gates

#### Scenario: Dirty tree
- **WHEN** the stamp names HEAD but the tree has uncommitted changes
- **THEN** `-if-stale` runs the gates

### Requirement: No partial runs
`-if-stale` combined with `-only` SHALL be refused, since a partial run never leaves a stamp.

#### Scenario: Only and if-stale
- **WHEN** both flags are given
- **THEN** the command fails with a reason and runs nothing
