# Skill artifact requirements

## Requirement: Canonical skill source

`skills/specsync/SKILL.md` SHALL be the canonical authored skill. Every bundled
or compatibility copy SHALL be byte-identical at release time.

### Scenario: CLI behavior changes

- **WHEN** a flag or lifecycle behavior is added to the canonical skill
- **THEN** the generated CLI, npm, and Claude skill copies contain the same text
- **AND** automated validation fails if any copy drifts

### Scenario: npm package is packed

- **WHEN** the npm tarball is prepared
- **THEN** it contains the current canonical specsync skill
- **AND** the skill documents `-close-completed`, `stage:complete`, and monotonic checkbox reconciliation
