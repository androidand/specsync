# store-resolution

## Purpose
Defines how specsync locates its OpenSpec spec root when planning lives in a
standalone store repo, and how a change declares which code repositories it
targets once the working directory no longer implies one.

## ADDED Requirements

### Requirement: Store declared by a code repo is resolved
specsync SHALL resolve its spec root to a store's registered location when the
working directory's `openspec/config.yaml` declares `store: <id>`.

#### Scenario: Bare invocation in a store-configured repo
- **WHEN** `specsync changes` runs in a code repo whose `openspec/config.yaml` declares `store: exopen-plans`
- **THEN** the changes listed are those in the store
- **AND** no `openspec/changes/` directory is required in the code repo

#### Scenario: Explicit flag wins over declared store
- **WHEN** `-openspec <path>` is passed in a repo that also declares `store:`
- **THEN** the flag's path is used

### Requirement: An unresolvable spec root fails loudly
specsync SHALL exit non-zero with a diagnostic naming what it looked for when no
spec root can be resolved, and SHALL NOT report an empty result set as success.

#### Scenario: Declared store is not registered
- **WHEN** `openspec/config.yaml` declares a store that is not in the machine registry
- **THEN** specsync exits non-zero naming the store id and the registration command
- **AND** it does not print an empty changes table

### Requirement: A change declares its target repositories
specsync SHALL read an optional `targets` array of provider keys from a change's
committed `specsync.yml`, and when it is present SHALL resolve the target
repository from it rather than from the working directory's git remote. The
declaration SHALL live outside the gitignored `.specsync/` cache so that it
survives a clone.

#### Scenario: Targets survive a clone
- **WHEN** a change declaring targets is committed and cloned by a teammate
- **THEN** the teammate's specsync resolves the same target repository
- **AND** the declaration is not part of the gitignored projection cache

#### Scenario: Target overrides the ambient remote
- **WHEN** a change declares `targets: ["github:ExopenGitHub/FusionHub"]` and is synced from the store's own directory
- **THEN** the issue is created in `ExopenGitHub/FusionHub`
- **AND** not in the store's own repository

#### Scenario: Explicit flag still wins
- **WHEN** `-repo ExopenGitHub/portal` is passed for a change targeting FusionHub
- **THEN** the flag's repo is used

#### Scenario: No targets preserves current behaviour
- **WHEN** a change declares no targets
- **THEN** the repository resolves via gh detection and the git remote, as before

### Requirement: One change projects to each of its targets
specsync SHALL maintain one issue per target repository when a change declares
several targets, cross-link those issues, and key each ref by its target.

#### Scenario: Backend and frontend halves stay linked
- **WHEN** a change targets both `github:ExopenGitHub/FusionHub` and `github:ExopenGitHub/portal`
- **THEN** an issue exists in each repository, each carrying the change's identity marker
- **AND** each issue links to the other
- **AND** `refs.json` holds both under their own provider keys

### Requirement: A store is not swept by an unscoped sync
specsync SHALL require `-change` or an explicit `-all` before writing when the
resolved spec root is a store.

#### Scenario: Unscoped sync against a store is refused
- **WHEN** bare `specsync` runs with a store as its spec root and neither `-change` nor `-all` is given
- **THEN** it exits non-zero explaining that a store spans repositories
- **AND** no provider write occurs
