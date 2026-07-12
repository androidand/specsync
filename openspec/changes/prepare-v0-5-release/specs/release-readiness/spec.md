# v0.5 release readiness requirements

## Requirement: Promotional claims match shipped behavior

Every feature claim in README, site content, and release notes SHALL name
behavior present in the tagged binary and covered by a test or smoke check.

### Scenario: Dry-run is promoted

- **WHEN** promotional copy describes dry-run support
- **THEN** it names only commands whose flag parsing and no-mutation behavior are implemented

### Scenario: Completion lifecycle is promoted

- **WHEN** the site describes `stage:complete` or `-close-completed`
- **THEN** one sync performs reconciliation, stage update, and close/reopen consistently

## Requirement: Release artifacts agree

The corrective Git tag, stamped Go binary, npm package version, bundled skill,
checksums, and promotional version SHALL all identify v0.5.1 and describe the
same CLI. The already-published v0.5.0 tag SHALL NOT be moved.

### Scenario: npm publication begins

- **WHEN** the npm publish job runs
- **THEN** GitHub release binaries and checksums already exist
- **AND** the packed npm skill matches the canonical skill
- **AND** a supported-platform install produces `specsync version` reporting v0.5.1
