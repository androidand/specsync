# Tasks: Beads as a work provider

- [x] Add `provider/beads`: implement `WorkProvider` (Name/Push/Find) + `IssueReader` (Get) + `TaskStateReader` (TaskStates) via an injectable `bd` runner; std-lib only, no Dolt
- [x] `-provider beads` selection in the CLI; dry-run prints the `bd` commands and makes zero writes
- [x] Identity: write the shared `specsync:change=<slug>` marker into bead descriptions; cache bead ids in `.specsync/refs.json` (gitignored); rebuild via marker scan; match children to tasks by normalized title
- [x] Inbound reconcile via the shared monotonic union (`TaskStateReader` → `mergeTaskState`); OpenSpec owns wording/order; the GitHub path stays byte-identical
- [x] Tests with a fake `bd` runner: create epic + children, create-only re-push, `TaskStates` excludes the epic, `Find` returns the epic, end-to-end reconcile from a closed bead
- [x] Outbound status projection: close a child bead when its task is checked (monotonic — never reopens); an already-checked task is created then closed
- [ ] Optional auto-detection gate: activate Beads when `bd` is on PATH or `.beads/` exists (today: explicit `-provider beads` only)
- [ ] Docs: provider contract + Beads activation in README/SKILL; state memory-out-of-scope and the single-source-of-truth rule explicitly
