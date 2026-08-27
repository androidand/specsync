# change-body-composition Specification

## Purpose
Define what sections a synced issue body contains, in what order, and how a
section that would push the body over the tracker's size limit overflows to a
linked comment instead of being dropped, truncated, or causing the sync to
fail outright. This formalizes behavior that otherwise lives only as implicit
logic in `WorkItemFor`, giving contributors something to check a new section
against instead of reverse-engineering `sync.go`.

## Requirements
### Requirement: Section set and ordering
specsync SHALL compose a synced issue body from the following sections, in
this order, each present only when its source is non-empty: `Body` (the
proposal), `## Original ask`, `## Design notes`, `## Discoveries`, `## Tasks`,
`## Plan changes`, and the dependency sections (`## Related`, `## Blocked
by`, `## Blocks`). The order follows the authoring timeline of a change,
oldest context to newest.

#### Scenario: All optional sections present
- **WHEN** a change has `original-ask.md`, `design.md`, `discoveries.md`, and `tasks.md` all non-empty
- **THEN** the rendered body contains `## Original ask` before `## Design notes`, `## Design notes` before `## Discoveries`, and `## Discoveries` before `## Tasks`

#### Scenario: Optional section omitted when empty
- **WHEN** a change has no `design.md` (or an empty one)
- **THEN** the rendered body contains no `## Design notes` section at all

### Requirement: Design notes sync into the issue body
specsync SHALL load `design.md`, when present, into `Change.DesignNotes` and
render it as a `## Design notes` section, the same treatment already given to
`original-ask.md` and `discoveries.md`. `specsync pull` SHALL extract a
`## Design notes` section back into local `design.md`, write-once: an
existing local `design.md` is never overwritten by a pull, because local
content may already be richer than what was last synced (e.g. mid-edit,
not yet pushed).

#### Scenario: design.md syncs into the issue
- **WHEN** a change has a non-empty `design.md`
- **THEN** the synced issue body contains a `## Design notes` section with its content

#### Scenario: Pull does not clobber richer local design.md
- **WHEN** local `design.md` already exists and an issue is pulled that also carries a `## Design notes` section
- **THEN** the local `design.md` file is left unchanged

#### Scenario: First pull seeds design.md from the issue
- **WHEN** no local `design.md` exists and the pulled issue carries a `## Design notes` section
- **THEN** `design.md` is created locally with that section's content

### Requirement: Oversized section overflows to a linked comment
specsync SHALL post design.md's content as a marked issue comment instead of
inlining it, and leave a linked stub in the body's Design notes section,
whenever the rendered body would otherwise exceed the tracker's body size
limit. A re-sync SHALL update that same marked comment idempotently rather
than creating a duplicate.

#### Scenario: Design notes overflow to a comment
- **WHEN** a change's rendered body, with `design.md` inlined, would exceed the tracker's body size limit
- **THEN** the issue body's `## Design notes` section is a stub linking to a comment
- **AND** that comment contains `design.md`'s full content and its identity marker

#### Scenario: Re-syncing an oversized change updates the same comment
- **WHEN** a change already synced via the overflow path is synced again with the same or updated `design.md` content
- **THEN** the existing marked comment is updated in place
- **AND** no second design-notes comment is created

### Requirement: Comment overflow previews under -dry-run
`-dry-run` SHALL preview the comment it would create or update for an
oversized `design.md`, the same way it already previews body and marker
writes, without making any network call.

#### Scenario: Dry run previews the overflow comment
- **WHEN** `-dry-run` syncs a change whose rendered body would overflow because of `design.md`
- **THEN** the dry-run output shows the comment content that would be created or updated
- **AND** no issue, comment, or label write is made

### Requirement: Shrinking design.md moves it back inline
specsync SHALL move design.md's content back into the issue body's Design
notes section, and mark the earlier overflow comment stale rather than
deleting it, once a change's rendered body no longer exceeds the size limit.

#### Scenario: Design notes shrink back below the limit
- **WHEN** a previously oversized change's `design.md` has shrunk enough that the rendered body no longer exceeds the size limit
- **THEN** the issue body's `## Design notes` section is inlined again in full
- **AND** the earlier overflow comment is edited to note it is stale, not deleted

#### Scenario: No stale rewrite once already marked
- **WHEN** a design-notes overflow comment has already been marked stale on a prior sync
- **THEN** a later sync with design.md still fitting inline makes no further write to that comment
