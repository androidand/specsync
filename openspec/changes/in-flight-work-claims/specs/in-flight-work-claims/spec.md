# in-flight-work-claims

## ADDED Requirements

### Requirement: Record an in-flight claim on a change
specsync SHALL let an agent record that it is actively working a change,
capturing worktree path, branch, agent identifier, expected file globs, and an
expiry. Claims are stored in the gitignored `.specsync/` cache and projected onto
the tracker item so they are visible without repo access. Claims are advisory:
they make concurrent work observable, they do not prevent writes.

#### Scenario: An agent claims a change before starting
- **WHEN** an agent runs `specsync claim -change fix-step-source-consistency`
- **THEN** the claim records the cwd's worktree path and branch
- **AND** the claim is listed by `specsync claims`
- **AND** the claim expires after its TTL so a crashed agent does not hold it forever

#### Scenario: Releasing a claim
- **WHEN** an agent runs `specsync release -change <id>`
- **THEN** the claim no longer appears in `specsync claims`

### Requirement: Detect overlapping claims before work begins
specsync SHALL fail a `claim` whose file globs intersect a live claim on another
change, naming the holding change, agent and worktree. An explicit `-force`
overrides, because reassigning work is legitimate.

#### Scenario: Two agents about to write the same module
- **WHEN** one agent holds a claim covering `shared/src/application/**`
- **AND** a second agent claims globs intersecting that path
- **THEN** the second claim fails and names the holder's change, agent and worktree
- **AND** `-force` allows it to proceed deliberately

#### Scenario: Two agents in one worktree
- **WHEN** two live claims name the same worktree path
- **THEN** `specsync claims` reports the shared worktree as a collision

### Requirement: Warn about work held in volatile or uncommitted state
specsync SHALL warn when a claimed worktree lives under a system temp directory,
and SHALL flag claims whose worktree holds uncommitted changes, including how
long they have been uncommitted. The hazard is uncommitted work in a path the OS
may reclaim; either condition alone is survivable.

#### Scenario: Claiming a worktree under /tmp
- **WHEN** a claim's worktree resolves under `/tmp`, `$TMPDIR` or `/private/tmp`
- **THEN** specsync warns that the worktree is volatile

#### Scenario: Uncommitted work in a volatile worktree
- **WHEN** a claimed volatile worktree has uncommitted changes
- **THEN** `specsync claims` flags it and reports the age of those changes

### Requirement: Determine landed state without relying on ancestry
specsync SHALL record the merge or squash commit for a change once its tracker
item closes, and SHALL answer "has this landed?" from that record rather than
from commit ancestry, which squash merges break.

#### Scenario: Content landed via squash, issue still open
- **WHEN** a change's content is present on the default branch
- **AND** none of its branch commits are ancestors of that branch
- **AND** its tracker item is still open
- **THEN** specsync reports the change as landed-but-open and names the squash commit
- **AND** warns that the source branch may now be behind the default branch

#### Scenario: A squash labelled as a merge
- **WHEN** a single-parent commit's subject begins with `merge:`
- **THEN** specsync reports it as a squash presented as a merge
- **AND** does not attempt to rewrite history

### Requirement: Attach provenance to verification claims
specsync SHALL allow a verification note to record the commit SHA it was measured
at, and SHALL render that note as stale when the SHA is no longer the branch tip,
so a green result cannot be carried forward past the change that invalidated it.

#### Scenario: Verification measured at an older commit
- **WHEN** a change records "tests green" at commit `abc123`
- **AND** the branch tip has since moved to `def456`
- **THEN** the verification note is rendered as stale, naming both commits

#### Scenario: Checks bypassed on merge
- **WHEN** a pull request is merged with required checks bypassed
- **THEN** specsync notes the bypass on the change's tracker item
