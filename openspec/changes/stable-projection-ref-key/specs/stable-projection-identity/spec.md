# stable-projection-identity

## ADDED Requirements

### Requirement: Repo-stable ref-cache key
The GitHub provider SHALL key the ref cache by the concrete target repository,
`"github:owner/repo"`, regardless of whether the repo was supplied via `-repo` or
auto-detected from the git remote. Two providers targeting the same repository SHALL
produce the same cache key.

#### Scenario: Auto-detected and explicit-repo providers share a key
- **WHEN** a change is bound by `specsync pull -issue N` (repo-scoped provider) and later synced by `specsync -change <change>` without `-repo`
- **THEN** both operations resolve the same `"github:owner/repo"` cache key
- **AND** the later sync sees the cached ref (`hadRef` is true) and updates the existing issue

#### Scenario: No duplicate when the marker is absent
- **WHEN** the cached ref resolves to an issue whose body has no identity marker
- **THEN** sync updates that issue via the cached ref
- **AND** it does not create a new issue

### Requirement: Backward-compatible, self-healing ref lookup
When resolving a change's ref, specsync SHALL first look up the canonical
`"github:owner/repo"` key and, if absent, fall back to the legacy bare `"github"`
key. A ref found under the legacy key SHALL be re-saved under the canonical key on
the next persist, and SHALL NOT trigger a create.

#### Scenario: Legacy cache keeps working
- **WHEN** an existing `refs.json` holds a binding under the bare `"github"` key
- **THEN** a sync finds it and updates the linked issue
- **AND** the ref is rewritten under the canonical `"github:owner/repo"` key

### Requirement: Pull persists the identity marker on the source issue
When `pull` links a change to an existing issue `#N`, specsync SHALL write the
identity marker `<!-- specsync:change=<slug> -->` into that issue's body (idempotent
upsert), so the link is rediscoverable by `Find` even if the local ref cache is lost.

#### Scenario: Rediscovery after cache loss
- **WHEN** a change is pulled from issue `#N`, then its `.specsync/` cache is deleted, then `specsync -change <change>` runs
- **THEN** `Find` locates `#N` by its marker
- **AND** sync updates `#N` instead of creating a duplicate

#### Scenario: Pull dry-run previews the marker edit
- **WHEN** `specsync pull -issue N -dry-run` runs
- **THEN** it reports the marker it would add to issue `#N`
- **AND** it makes no GitHub write
