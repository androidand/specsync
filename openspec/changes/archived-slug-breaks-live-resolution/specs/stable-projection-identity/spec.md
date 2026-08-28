## MODIFIED Requirements

### Requirement: Pull persists the identity marker on the source issue

When `pull` links a change to an existing issue `#N`, specsync SHALL write the
identity marker `<!-- specsync:change=<slug> -->` into that issue's body (idempotent
upsert), so the link is rediscoverable by `Find` even if the local ref cache is lost.
When a change's on-disk slug later changes (archiving's date-prefix rename is the
one case this happens today), a live `Find`/`ResolveLiveRefs` lookup by the new slug
SHALL still resolve the existing issue rather than treating it as unresolvable.

#### Scenario: Rediscovery after cache loss

- **WHEN** a change is pulled from issue `#N`, then its `.specsync/` cache is deleted, then `specsync -change <change>` runs
- **THEN** `Find` locates `#N` by its marker
- **AND** sync updates `#N` instead of creating a duplicate

#### Scenario: Pull dry-run previews the marker edit

- **WHEN** `specsync pull -issue N -dry-run` runs
- **THEN** it reports the marker it would add to issue `#N`
- **AND** it makes no GitHub write

#### Scenario: Rediscovery survives an archive rename with no ref cache

- **WHEN** a change is archived (its folder renamed to a date-prefixed slug) after its issue's marker was written under the pre-archive slug, and no local `.specsync/` ref cache exists
- **THEN** `Find`/`ResolveLiveRefs` for the new (archived) slug still resolves the same issue
- **AND** neither a live sync nor `changelog -resolve-refs` creates a duplicate issue or drops the change from the changelog
