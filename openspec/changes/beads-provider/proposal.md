# Beads as a work provider

specsync is tracker-agnostic: the `WorkProvider` interface projects an OpenSpec
change into whatever tracker a project uses. GitHub is the default; Beads (`bd`)
is a legitimate second tracker — an issue/task graph some projects run — so
specsync should target it as a first-class provider, exactly as it would Jira or
Linear. This change adds that provider.

Memory is explicitly **not** specsync's concern. Beads happens to also offer
long-term memory (`bd remember` / `bd prime`); whether and how a project uses
that is the tool's own business. specsync neither reads nor writes Beads memory —
it synchronizes tasks, nothing else.

## Principle: edges, not nodes; one source of truth

- **OpenSpec** owns intent, scope, and task wording/order — the single source of
  truth.
- A provider is a **projection** of that truth. Beads is a projection like any
  other; it never becomes a second authority.
- specsync synchronizes the *correspondence* between spec and tracker. It does
  not own the work, orchestrate it, or remember it — not a control plane, not a
  memory tool.

## What

A `beads` provider behind `WorkProvider`, driven by the `bd` CLI (shell-out only,
std-lib only, never Dolt internals). It differs from the GitHub provider only in
*shape*, because Beads models tasks as a graph of items rather than checkboxes in
one issue body:

1. **Identity.** The shared `<!-- specsync:change=<slug> -->` marker is written
   into every bead's description (epic and children), found via
   `bd list --desc-contains`. The bead id is cached in `.specsync/refs.json`,
   never committed, and rebuilt from the marker on cache loss — exactly the
   GitHub identity model.

2. **Shape.** One epic bead per change (the ref anchor); one child bead per task,
   matched to its task by normalized title. This shape difference is precisely
   what the `TaskStateReader` capability abstracts, so the reconcile engine stays
   tracker-agnostic instead of GitHub-shaped.

3. **Reconcile — uniform, not inverted.** Done-state read from the child beads
   merges into `tasks.md` by the **same monotonic union every provider uses**: a
   task is done if either side has it done, OpenSpec always owns wording/order,
   and local progress is never reverted. (Earlier drafts described Beads as
   "owning status"; that assumed agents work the bead graph directly, which is
   out of scope. The union rule is uniform across all providers.)

## Scope

- `provider/beads`: `WorkProvider` (Name/Push/Find) + `IssueReader` (Get) +
  `TaskStateReader` (TaskStates), via an injectable `bd` runner.
- `-provider beads` selection; dry-run prints the `bd` commands and makes zero
  writes.
- Identity marker + `refs.json` caching + marker-scan rebuild; children matched
  by normalized title.
- Create-only `Push` for v1 (creates the epic + any missing child beads; never
  re-titles or reopens).
- Outbound status projection (follow-up): close a child bead when its task is
  checked, so the projection reflects done-state both ways.

## Non-goals

- **Not a memory tool.** specsync never calls `bd remember` / `prime` / `gc`;
  Beads memory is the tool's own concern.
- **Not a second source of truth.** Beads receives wording/scope and contributes
  only done-state inbound; it never overrides OpenSpec.
- **Not a control plane.** No multi-agent `--claim`, no runtime orchestration —
  that is the executor's job, not specsync's at sync time.
- No Dolt coupling, no `bd` server mode.
- Not the default provider; GitHub stays default.
