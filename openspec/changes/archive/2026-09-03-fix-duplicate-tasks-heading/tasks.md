# Tasks

- [x] 1. Strip a leading H1 from `c.TasksMarkdown` in `WorkItemFor`
      (`sync.go`) before appending it after `## Tasks`.
      Validation: a fixture whose tasks.md starts with `# Tasks` renders a
      body with exactly one "Tasks" heading; a fixture without a leading H1
      is unaffected.

- [x] 2. Regression test against the real demo issue shape (a tasks.md
      identical to what `specsync agent-help`/existing changes conventionally
      produce: `# Tasks\n\n- [ ] ...`).
