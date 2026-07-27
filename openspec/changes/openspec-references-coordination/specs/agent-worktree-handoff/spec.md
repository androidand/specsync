# agent-worktree-handoff

## ADDED Requirements

### Requirement: Produce a durable coordination handoff

specsync SHALL provide a read-only coordination report joining an active
OpenSpec change with its tasks, references, worksets, synced tracker/provider
identity, Git branch/worktree state, and related or blocking changes.

#### Scenario: Handoff between agents

- **WHEN** an agent requests a handoff report for an active change
- **THEN** the report identifies the change, issue/provider, branch, worktree,
  base revision, current revision, claimed tasks, owned paths, related changes,
  blockers, and next safe action
- **AND** facts are distinguishable from recommendations

### Requirement: Detect coordination drift without mutation

The report SHALL flag stale bases, duplicate active worktrees or branches,
overlapping claimed paths, dirty worktrees, unpushed commits, missing PRs, and
unchecked verification evidence without modifying Git, OpenSpec, or a tracker.

#### Scenario: Two agents implement the same task

- **WHEN** two active worktrees claim the same change or owned path
- **THEN** the report lists both worktrees and identifies the overlap
- **AND** it recommends a handoff or task split rather than silently choosing one

#### Scenario: Branch drift before merge

- **WHEN** a feature branch is behind its recorded base or current main
- **THEN** the report marks it stale and prints the rebase/merge check required
  before review

### Requirement: Preserve provider identity across related changes

specsync SHALL show the selected work provider and synced issue reference for
each change and SHALL report an actionable mismatch when related changes cannot
be linked because they use different providers or lack synced references.

#### Scenario: GitHub and Beads changes are related

- **WHEN** two changes have the same local relationship but one is synced to
  GitHub and the other to Beads
- **THEN** the report identifies both provider identities
- **AND** it explains the explicit-provider synchronization required to create a
  cross-change tracker relationship

### Requirement: Surface deterministic merge order and cleanup

specsync SHALL report a deterministic topological order from directed change
dependencies and SHALL provide post-merge cleanup recommendations for branches,
worktrees, OpenSpec archival, and issue closure. Recommendations SHALL remain
non-mutating unless a separate explicit command is invoked.

#### Scenario: Backend blocks frontend

- **WHEN** a frontend change is blocked by a backend change
- **THEN** the report orders the backend first and identifies the frontend as
  waiting for the backend merge and rebase

#### Scenario: Merged change remains locally active

- **WHEN** a PR is merged but its branch, worktree, or OpenSpec change remains
  present
- **THEN** the report lists cleanup actions without executing them
