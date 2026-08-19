## ADDED Requirements

### Requirement: Create or converge a coordination epic
specsync SHALL create a `type:epic` issue for `specsync epic <title> --repo
owner/name [--child <slug|owner/repo#N|url>]...`, or converge onto an
existing one, keyed by an identity marker derived from the normalized title.
It SHALL NOT create a second epic issue for the same normalized title on a
repeated invocation.

#### Scenario: First invocation creates the epic
- **WHEN** `specsync epic "Feature X" --repo org/planning` runs and no epic
  with that normalized title exists yet
- **THEN** specsync creates a new issue in `org/planning` labeled `type:epic`
  and `specsync`
- **AND** prints the epic's URL

#### Scenario: Re-running with the same title converges
- **WHEN** `specsync epic "Feature X" --repo org/planning` runs again with
  the same title
- **THEN** specsync finds the existing epic (via its identity marker) instead
  of creating a second issue
- **AND** updates its body in place rather than duplicating content

### Requirement: Attach children by slug or by issue reference, cross-repo
`--child` SHALL accept a local OpenSpec change slug or an existing issue
reference (`owner/repo#N`, bare `#N` resolved against `--repo`, or a full
issue URL), and multiple `--child` flags SHALL be attachable in one
invocation, spanning different repos.

#### Scenario: Slug child is synced first
- **WHEN** `--child <slug>` names a local change with no synced ref yet
- **THEN** specsync syncs that change to create its issue before attaching it
  to the epic

#### Scenario: Cross--repo issue-reference child
- **WHEN** `--child owner/other--repo#12` is given
- **THEN** specsync attaches issue `#12` in `owner/other--repo` to the epic
  without creating any local change directory for it

#### Scenario: Mixed children in one call
- **WHEN** one invocation passes both a slug and an `owner/repo#N` reference
  as separate `--child` flags
- **THEN** both are attached to the same epic in the same run

### Requirement: Degrade gracefully to a managed `## Related` cross-link
Until native GitHub sub-issue attachment is available, each child SHALL be
wired to the epic (and the epic to each child) via the same managed `##
Related` section used elsewhere, upserted idempotently in both directions.

#### Scenario: Related section on the epic
- **WHEN** children are attached in degraded mode
- **THEN** the epic body's `## Related` section lists every child's issue URL

#### Scenario: Related section on each child
- **WHEN** children are attached in degraded mode
- **THEN** each child issue's body gets a `## Related` entry pointing back at
  the epic, upserted in place rather than appended as a duplicate on re-run

### Requirement: Idempotent re-run changes nothing when nothing changed
specsync SHALL NOT create a duplicate issue or duplicate `## Related` entries,
and SHALL NOT issue unnecessary GitHub writes, when `specsync epic` is
invoked again with the same title and the same set of children.

#### Scenario: No-op re-run
- **WHEN** `specsync epic` is invoked twice in a row with identical arguments
- **THEN** the second run makes no additional GitHub issue-create calls and
  leaves the `## Related` sections unchanged in content
