# release-impact

## ADDED Requirements

### Requirement: Infer an advisory SemVer bump from multiple signals
specsync SHALL infer an advisory bump (`none`, `patch`, `minor`, or `major`) by
combining Conventional Commit types, breaking markers/footers, and OpenSpec
requirement deltas, taking the maximum impact across all signals. The bump is a
recommendation; specsync does not perform it.

#### Scenario: A feature commit implies minor
- **WHEN** the range contains a `feat` commit and no stronger signal
- **THEN** the advised bump is `minor`

#### Scenario: A breaking change overrides everything
- **WHEN** the range contains a commit marked breaking alongside several `fix` commits
- **THEN** the advised bump is `major`

#### Scenario: Chore-only range implies no release
- **WHEN** the range contains only `chore`, `docs`, and `test` commits
- **THEN** the advised bump is `none`

### Requirement: Treat OpenSpec requirement deltas as a release signal
specsync SHALL derive release impact from a change's requirement deltas, obtained
from the `openspec` CLI (`show --json --deltas-only`): a `REMOVED` operation
contributes `major`, an `ADDED` operation contributes `minor`, and a `MODIFIED`
operation contributes `patch`. specsync SHALL NOT compute deltas by parsing spec
markdown itself.

#### Scenario: A refactor that removes a requirement
- **WHEN** a change's commits are all `refactor` but `openspec show --json --deltas-only` reports a `REMOVED` delta
- **THEN** the advised bump is `major`

#### Scenario: Deltas unavailable without the openspec CLI
- **WHEN** the `openspec` CLI is not available
- **THEN** the requirement-delta signal is reported as unavailable
- **AND** the bump is inferred from commit signals alone, not guessed from raw files

### Requirement: Compute release deltas as a join with git history
specsync SHALL determine which changes' deltas count toward a release range by
joining git history with OpenSpec state: the changes completed or archived within
`[since, until]`. For a still-active change it SHALL read deltas via `openspec
show`; for a change archived within the range it SHALL reconstruct deltas from the
change's spec files at the git ref where it existed. specsync SHALL NOT treat the
working-tree deltas at `HEAD` as the whole release signal.

#### Scenario: Current unreleased work since the last tag
- **WHEN** the range is "since the last tag" and the contributing changes are still active
- **THEN** their deltas are read directly via `openspec show --json --deltas-only`

#### Scenario: A historical range spanning an archive
- **WHEN** the range includes a commit that archived a change
- **THEN** that change's deltas are reconstructed as of its archive, not from the current tree

### Requirement: Define delta semantics before the first baseline
Until a project has archived its first change (no accepted baseline), specsync
SHALL treat every requirement delta as `ADDED` and SHALL NOT infer `MODIFIED` or
`REMOVED`. In that state the spec-delta signal contributes at most `minor`; a
`major` can come only from a commit breaking marker.

#### Scenario: Pre-baseline project
- **WHEN** `openspec list --specs` reports no accepted specs
- **THEN** spec-delta signals are all `ADDED` and advise at most `minor`
- **AND** the report notes the baseline does not exist yet

### Requirement: Explain every recommendation
specsync SHALL attach a human-readable reason to each signal that contributed to
the recommended bump.

#### Scenario: Reasons accompany the bump
- **WHEN** a `minor` is advised because of a `feat` commit and an added requirement
- **THEN** the output lists both as reasons

### Requirement: A fixed default mapping, no configuration yet
specsync SHALL use a fixed default type→impact mapping and SHALL NOT require a
configuration file. Project-specific overrides are deferred until a real need
appears.

#### Scenario: Defaults apply with no config present
- **WHEN** no `specsync.json` exists
- **THEN** the default mapping (`feat`→minor, `fix`→patch, others→none, breaking→major) applies
