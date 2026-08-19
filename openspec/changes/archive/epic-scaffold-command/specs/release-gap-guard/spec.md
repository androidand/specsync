## ADDED Requirements

### Requirement: Distinguish a dev build from a published release
`specsync --version` SHALL report whether the running binary is a published
release or a local/dev build, using build info embedded at build time
(e.g. `git describe`), so a user can tell "my installed copy lags the repo"
from "I'm already on the latest release" in one command.

#### Scenario: Dev build reports as such
- **WHEN** the binary was built from a working tree without a release tag
  (or via `go run`/`go build` outside the release pipeline)
- **THEN** `specsync --version` reports it as a dev build, not a release
  version number

#### Scenario: Released build reports its version
- **WHEN** the binary was built by the release pipeline for a tagged version
- **THEN** `specsync --version` reports that exact released version

### Requirement: A release SHALL NOT ship with a complete change left unarchived
specsync SHALL fail a release build when a fully complete, already-shipped
change is still sitting unarchived in `openspec/changes/`, so a capability
cannot go out in a published release while its own planning artifacts claim
it is still in progress.

This is already implemented and already enforced, not new code from this
change: `specsync release-plan -fail-on-archive-candidates` reports every
complete change with commits in the release range that remains unarchived,
and `.github/workflows/release.yml`'s "Enforce OpenSpec archive hygiene" step
runs it before every tag's `goreleaser` step, failing the release job if any
are found. This requirement documents that existing, already-shipped
behavior for completeness — see design.md's correction.

#### Scenario: Archive candidate blocks the release
- **WHEN** a tag is pushed and a complete, shipped change remains unarchived
  in `openspec/changes/`
- **THEN** `release-plan -fail-on-archive-candidates` exits non-zero
- **AND** the release workflow fails before `goreleaser` runs, publishing
  nothing

#### Scenario: Nothing to archive
- **WHEN** a tag is pushed and no complete, shipped change remains unarchived
- **THEN** the hygiene check passes and the release proceeds
