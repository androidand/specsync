# issue-dependency-sync

## ADDED Requirements

### Requirement: Declare directed dependency edges in links.md
specsync SHALL parse `## Blocked by` and `## Blocks` sections in a change's
`links.md`, each listing issue references (`#N` / `owner/repo#N` / URL). These are
directed edges, distinct from the symmetric `## Related` section.

#### Scenario: A frontend change blocked by a backend issue
- **WHEN** a change's `links.md` has `## Blocked by` → `org/billing-service#3489`
- **THEN** specsync treats that backend issue as blocking this change's issue
- **AND** the dependency direction is preserved (not flattened to "related")

### Requirement: Project dependencies onto GitHub issue dependencies
On sync specsync SHALL make the issue's dependencies match `links.md`: a
`## Blocked by` entry is projected via `addBlockedBy`, and a `## Blocks` entry is
projected as the named issue being blocked by this one. References in another repo
are linked by node id.

#### Scenario: Blocked-by projected to GitHub
- **WHEN** a change with a `## Blocked by` entry is synced
- **THEN** the change's issue gains a GitHub dependency on the named blocker
- **AND** a blocker in a different repo is linked across repos

### Requirement: Reconcile dependencies both ways against a baseline
specsync SHALL reconcile the dependency edge set bidirectionally against a
last-synced baseline stored in the gitignored `.specsync/` cache. Each edge is
binary, so every case resolves without ambiguity: an edge in `links.md` but not the
baseline is pushed (`addBlockedBy`); a dependency on GitHub but not the baseline is
pulled into `links.md`; an edge in the baseline but missing from `links.md` is
removed on GitHub (`removeBlockedBy`); an edge in the baseline but missing on GitHub
is removed from `links.md`. After reconcile the baseline becomes the converged set.

#### Scenario: Local removal clears the dependency on GitHub
- **WHEN** a `## Blocked by` entry that was in the baseline is deleted from `links.md` and the change is synced
- **THEN** specsync removes the dependency on GitHub
- **AND** the baseline no longer records it

#### Scenario: A dependency added on GitHub is pulled into links.md
- **WHEN** a person adds a blocked-by dependency on GitHub (not in the baseline) and the change is synced
- **THEN** specsync writes it into `links.md` as a `## Blocked by` entry
- **AND** the dependency is not discarded

#### Scenario: Removal on GitHub clears the local entry
- **WHEN** a dependency that was in the baseline is removed on GitHub and the change is synced
- **THEN** specsync removes the `## Blocked by` entry from `links.md`
- **AND** does not re-push it

### Requirement: Surface dependency conflicts rather than pre-validating
specsync SHALL rely on GitHub to reject invalid dependencies (e.g. a cycle) and
SHALL surface that error, rather than maintaining its own cycle check.

#### Scenario: GitHub rejects a cycle
- **WHEN** projecting a dependency would create a cycle and GitHub returns an error
- **THEN** specsync reports the error
- **AND** does not silently drop the edge
