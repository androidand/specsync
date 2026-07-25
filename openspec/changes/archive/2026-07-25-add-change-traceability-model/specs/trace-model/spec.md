# trace-model

## ADDED Requirements

### Requirement: Represent a change's traceability graph
specsync SHALL model the relationships between an OpenSpec change and its
work items, pull requests, and commits as a graph of nodes and directed links.

#### Scenario: A change with linked artifacts
- **WHEN** a trace is resolved for a change that has an issue ref, two contributing commits, and one merged PR
- **THEN** the trace contains the change node linked to the work item, the two commit nodes, and the PR node

### Requirement: Record link provenance
Every link in a trace SHALL record how it was discovered: an issue-body marker,
a branch name, a commit footer, a PR body, the ref cache, or a `links.md` entry.

#### Scenario: Commit linked by a closing footer
- **WHEN** a commit closes an issue that is the projection of a change
- **THEN** the change↔commit link is present
- **AND** its provenance is recorded as a commit footer (via the issue)

### Requirement: Resolve over a change, a revision range, or an area
specsync SHALL resolve a trace for any of three scopes: a single change (by
slug), a revision range (by `since`/`until`), or an area (by path(s) and/or a
topic string). The scope value SHALL be general enough that inbound (planning)
and outbound (release) consumers share one resolver.

#### Scenario: Range scope
- **WHEN** a trace is requested for `--since v0.2.0 --until HEAD`
- **THEN** the trace includes the commits in that range and the changes they link to

#### Scenario: Area scope
- **WHEN** a trace is requested for an area (a path glob and/or a topic string)
- **THEN** the trace includes the changes, issues, commits, and PRs that touch those paths or match the topic

### Requirement: Area matching is deterministic and ordered
When resolving an area scope, specsync SHALL match by deterministic rules only,
never by semantic similarity or an LLM. A path entry is a shell glob matched
against changed files via `git log -- <glob>`; a topic entry is a
case-insensitive substring matched against OpenSpec change titles and proposal
text and against issue titles and bodies. Results SHALL be ordered exact-path
matches first, then topic matches, then by recency, breaking ties by commit date
descending and then by slug, so repeated runs are byte-identical.

#### Scenario: Topic is a substring, not a semantic match
- **WHEN** an area topic is `modal` and a change's proposal contains the word `modal`
- **THEN** that change is included
- **AND** a change about `dialog` with no `modal` substring is not included

#### Scenario: Stable ordering across runs
- **WHEN** two changes match the same topic with the same recency
- **THEN** they are ordered by slug, so repeated runs and `--json` output diff cleanly

### Requirement: Source pull-request nodes from resolved references
specsync SHALL add a pull-request node only when a PR is named by resolved
evidence — a commit footer or branch name, or a reference on the linked issue.
Reading a PR body to enrich the node requires the `gh` CLI and is best-effort;
the `pr-body` provenance SHALL be recorded only when a PR body was actually read,
otherwise the node carries the provenance of the reference that surfaced it.

#### Scenario: PR known only by a commit reference
- **WHEN** a commit footer references a pull request and `gh` is unavailable
- **THEN** the PR appears as a node linked from that commit
- **AND** its provenance is the commit footer, not a PR body

### Requirement: Never fabricate links
specsync SHALL only record links it can resolve from real evidence; an
unresolved relationship SHALL be reported as a gap, not invented.

#### Scenario: A feat commit with no discoverable change
- **WHEN** a `feat` commit references no issue, branch, or change marker that maps to a change
- **THEN** no change↔commit link is created
- **AND** the commit is reported as an unlinked contributor

### Requirement: Resolution is read-only
Resolving a trace SHALL NOT modify any file, Git object, or tracker item.

#### Scenario: Pure resolution
- **WHEN** a trace is resolved
- **THEN** no local file, commit, issue, or label is created or changed
