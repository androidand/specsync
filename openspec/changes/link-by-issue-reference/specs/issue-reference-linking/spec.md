# issue-reference-linking

## ADDED Requirements

### Requirement: Accept issue references as link arguments
specsync SHALL accept, as arguments to `link`, issue references in addition to
local change slugs: a bare `#N` or `N` (resolved against the `-repo` repo, else
the auto-detected one), `owner/repo#N`, or a full GitHub issue URL. Slugs and
references MAY be mixed in one invocation. The reference forms SHALL be the same
shorthands that `links.md` entries accept.

#### Scenario: Link two issues by shorthand
- **WHEN** `specsync link org/billing-service#3489 org/dashboard#4084` runs
- **THEN** each issue is cross-referenced to the other
- **AND** no local change directory is created for either

#### Scenario: A slug linked to a reference
- **WHEN** `specsync link my-change org/dashboard#4084` runs
- **THEN** the change `my-change` records the issue in its `links.md` as today
- **AND** issue `dashboard#4084` is edited to reference `my-change`'s issue

### Requirement: Cross-reference existing issues without a local spec
When a `link` argument is an issue reference, specsync SHALL resolve the existing
issue, upsert a `## Related` section in its body listing the other linked issues'
URLs, and push the edited body — writing no change directory and no `links.md` for
that reference.

#### Scenario: Reference links live only in the issue bodies
- **WHEN** two issues are linked purely by reference
- **THEN** both issue bodies gain a `## Related` section pointing at each other
- **AND** the working tree is unchanged (no new file is written)

#### Scenario: An unresolved reference is an error, not a create
- **WHEN** a referenced issue does not exist
- **THEN** `link` reports the error
- **AND** it does not create an issue

### Requirement: Manage the Related section idempotently in any body
specsync SHALL upsert the `## Related` section — replacing an existing section in
place rather than appending a duplicate — for both spec-rendered bodies and
fetched raw issue bodies, so the two paths cannot drift in format and re-running
`link` is safe.

#### Scenario: Re-running does not duplicate the section
- **WHEN** `link` is run twice on the same issues
- **THEN** each body contains exactly one `## Related` section
- **AND** the second run leaves the section content unchanged

#### Scenario: An existing Related section is replaced, not stacked
- **WHEN** an issue already has a `## Related` section and is linked to a new issue
- **THEN** the section is rewritten to include the new entry
- **AND** content after the section (later `##` headings) is preserved

### Requirement: Target each reference's own repo
specsync SHALL edit each referenced issue through a provider bound to that
reference's repo, so a single `link` invocation can span repositories. A bare `#N`
uses the `-repo` repo when set, otherwise the auto-detected remote.

#### Scenario: Cross-repo link in one call
- **WHEN** references in two different repos are linked in one `link` call
- **THEN** each issue is edited in its own repo
- **AND** neither edit targets the wrong repo

### Requirement: Dry-run previews reference edits without calling GitHub
`link -dry-run` SHALL, for each referenced issue, print the issue-edit it would
perform and the `## Related` block it would write, and SHALL make no GitHub calls.

#### Scenario: Preview a reference link
- **WHEN** `specsync link -dry-run owner/repo#1 owner/repo#2` runs
- **THEN** it prints the `## Related` block each issue would receive
- **AND** no issue is edited
