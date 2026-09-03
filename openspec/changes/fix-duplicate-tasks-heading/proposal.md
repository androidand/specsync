# Fix duplicated "Tasks" heading in synced issues

## Why

`WorkItemFor` renders `"\n\n## Tasks\n\n" + c.TasksMarkdown`, and by
convention `tasks.md` itself opens with its own `# Tasks` H1 (every
tasks.md in this repo does). The result is a rendered issue body with two
consecutive "Tasks" headings — found by inspection on issue #154, the demo
change for `tighten-synced-issue-bodies` (#153):

```
## Tasks

# Tasks

- [x] 1. ...
```

Pre-existing — predates the collapsed-rendering change, just went
unnoticed until Tasks became the section people actually look at by
default.

## Proposed Changes

- Strip a leading H1 from `c.TasksMarkdown` before appending it, the same
  `stripLeadingH1` helper already used for the Proposal section (`sync.go`).

## Non-Goals

- **Not** touching `pull.go` — pull never generates the `# Tasks` H1 itself,
  it's a purely local authoring convention, and stripping on render doesn't
  need a matching extraction change (the H1 was never a managed section
  boundary to begin with).

## Release Notes

Fixed a duplicated "Tasks" heading in synced issue bodies.
