# Audit archived changes against merged PRs

## Why

The specsync lifecycle is:

```
active → complete → archived
```

But "archived" ≠ "merged to master." There's no signal between the two. An agent
can `openspec archive` a change, then open a PR, then forget to merge it. The
archived change disappears from `openspec list`, the GitHub issue gets closed,
and the open PR sits there unnoticed — exactly what happened with 4 PRs in
brick-now (#123, #71, #46, #29) that were archived but never merged.

The root problem: specsync tracks the *planning* lifecycle (tasks done → archive)
but has no awareness of the *shipping* lifecycle (archive → PR → merge). The gap
is invisible because there's no audit surface.

## Release note

Add `specsync audit` — a read-only command that cross-references archived
OpenSpec changes against GitHub PRs to find archived changes whose PR was never
merged. Also add a new `shipped` stage that represents the final step in the
lifecycle: the PR has landed.

## What Changes

### 1. New `specsync audit` command

Read-only. No GitHub writes. Queries open PRs and archived changes, then reports
the gap.

```
specsync audit [flags]
```

For each archived change, it checks:
1. Is there an open PR for this change? → report as "unmerged"
2. Is there a merged PR for this change? → report as "shipped" (info only)
3. No PR at all? → report as "orphaned" (info only)

The primary concern is (1): archived changes with open PRs that should have been
merged.

PRs are matched to changes using the change slug in the branch name or PR title.
The PR body is also searched for the specsync marker comment for unambiguous
matching.

### 2. New `shipped` stage

A new canonical stage: `shipped`. It represents "the change is archived AND the
PR has merged." The full lifecycle becomes:

```
active → complete → archived → shipped
```

The `shipped` stage is set by the `audit` command when it confirms a merged PR,
and stored in `.specsync/metadata.json`. This prevents the same PR from being
reported as "unmerged" on subsequent runs.

### 3. Pre-archive guard (optional, follow-up)

A `specsync archive` subcommand (or a flag on `openspec archive`) that checks
for an open PR before allowing the archive. This is a follow-up because it
requires changes to the OpenSpec CLI, not specsync.

## Out of scope

- Automatic PR creation from archived changes.
- Changing the OpenSpec archive semantics.
- Closing PRs as a side effect of audit.
- Tracking merge status for non-GitHub providers (beads).

## Capabilities

- `pr-state-reader`: read PR state from GitHub (open/merged/closed)
- `audit-surface`: cross-reference archived changes against PR state
