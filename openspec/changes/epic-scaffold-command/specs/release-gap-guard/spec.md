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

### Requirement: Require publication before archiving a shipped capability
The release checklist SHALL require that a change reaching 16/16 (fully
complete) tasks is published in the distributed package before it is
archived, so a capability cannot be marked archived while still absent from
what installed copies actually receive.

#### Scenario: Publish-before-archive enforced
- **WHEN** a change's tasks reach full completion and the operator attempts
  to archive it
- **THEN** the checklist requires confirming the current release has been
  published before archiving proceeds

#### Scenario: Historical gap this prevents
- **WHEN** a capability like `link-by-issue-reference` reaches 16/16 in git
  but the published package still serves an older version
- **THEN** the checklist surfaces that gap instead of allowing the change to
  be archived while silently unavailable to installed copies
