# sub-issue-projection

## ADDED Requirements

### Requirement: Recognize an epic without a spec of its own
specsync SHALL treat a GitHub issue carrying the `type:epic` label as an epic — a
coordination shell — and SHALL NOT require or create an OpenSpec change for it.

#### Scenario: An epic is coordination-only
- **WHEN** an issue labeled `type:epic` is encountered as a `## Parent`
- **THEN** specsync does not expect a local change for it
- **AND** it does not author a proposal or tasks for the epic

### Requirement: Project a change's parent edge onto a GitHub sub-issue
specsync SHALL read a `## Parent` entry from a change's `links.md` and make that
change's issue a sub-issue of the named parent, using the GitHub sub-issues API
(`addSubIssue`), passing the child's issue URL (`subIssueUrl`) so a parent in a
different repository works.

#### Scenario: Child projected under its parent
- **WHEN** a change's `links.md` has `## Parent` → `owner/repo#10` and the change is synced
- **THEN** the change's issue is attached as a sub-issue of `owner/repo#10`
- **AND** a parent in a different repo is attached via its issue URL

### Requirement: Reconcile the parent edge both ways against a baseline
specsync SHALL reconcile the parent edge bidirectionally against a last-synced
baseline stored in the gitignored `.specsync/` cache. Because the edge is binary,
each case resolves without ambiguity: an edge in `links.md` but not the baseline is
pushed to GitHub; a parent on GitHub but not the baseline is pulled into
`links.md`; an edge in the baseline but missing from `links.md` is removed on
GitHub; an edge in the baseline but missing on GitHub is removed from `links.md`.
After reconcile the baseline is updated to the converged set.

#### Scenario: Local removal detaches the sub-issue
- **WHEN** the `## Parent` entry is deleted from `links.md` (it was in the baseline) and the change is synced
- **THEN** specsync detaches the sub-issue on GitHub
- **AND** the baseline no longer records it

#### Scenario: A parent attached on GitHub is pulled into links.md
- **WHEN** a person attaches this issue under a parent on GitHub (not in the baseline) and the change is synced
- **THEN** specsync writes the parent into `links.md` as a `## Parent` entry
- **AND** the edge is not discarded

#### Scenario: Removal on GitHub clears the local entry
- **WHEN** a parent that was in the baseline is detached on GitHub and the change is synced
- **THEN** specsync removes the `## Parent` entry from `links.md`
- **AND** does not re-push the edge

### Requirement: Roll up the epic body from its sub-issues
specsync SHALL keep an epic issue's body roll-up in sync with its live
`subIssuesSummary` (total and completed), without overwriting a child sub-issue's
own body.

#### Scenario: Epic reflects child completion
- **WHEN** an epic has three sub-issues, one closed
- **THEN** its roll-up reports the total and completed counts from `subIssuesSummary`
- **AND** each child's body remains driven by its own change
