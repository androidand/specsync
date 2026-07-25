# launch-readiness

## ADDED Requirements

### Requirement: The README documents the full shipped CLI surface
The README SHALL document every subcommand the released binary accepts
(`sync`, `pull`, `scan`, `trace`, `link`, `release-plan`, `install-skill`)
with at least one runnable example each, so a first-time visitor can use the
tool without reading source code.

#### Scenario: A stranger looks up a shipped subcommand
- **WHEN** a reader searches the README for any subcommand handled in `cmd/specsync/main.go`
- **THEN** they find a usage example and a one-line description of that subcommand

### Requirement: The npm package carries discoverable metadata
The published npm package SHALL include `keywords`, `author`, `bugs`, and
`repository` fields so the registry page is searchable and issues are
reportable.

#### Scenario: A visitor lands on the npm registry page
- **WHEN** they view the package on npmjs.com
- **THEN** keywords, author, a bug-report link, and the source repository are all shown

### Requirement: The binary reports its own version
The specsync binary SHALL print its release version via a `version` subcommand
or `-version` flag, with the value injected at release build time.

#### Scenario: A user checks which version is installed
- **WHEN** they run `specsync version`
- **THEN** the released semver (or `dev` for local builds) is printed and the process exits 0
