# Two-way reconcile

specsync is push-only: `tasks.md` is rendered onto the issue, and anything edited
on the issue is overwritten on the next sync. But the issue checkbox is exactly
where a teammate ticks off work without opening the repo. So in real use, task
*state* churns on both sides, and specsync silently discards the issue side.

Making sync bidirectional is not "pick a winner." A spec is three layers with
three rightful owners, and conflating them is what makes a single-winner rule
wrong. This change reconciles each layer in its own direction.

## Policy

```
Original ask   →  preserved   (neither side overwrites; see living-plan)
Plan / tasks   →  spec wins    (active change is the source of truth)
Checkbox state →  issue wins   (pulled back into tasks.md)
```

- **Original intent** is anchored by `living-plan`'s `## Original ask` block and
  is read-only on sync, so the spec winning the plan can never erase the intent
  the work started from.
- **Task wording** — add / drop / replace / reorder — is authored in `tasks.md`.
  The issue is a projection; spec wins. Tasks present on the issue but absent from
  `tasks.md` are treated as removed, not re-added.
- **Checkbox state** — done vs not-done — is authoritative on the issue. On sync,
  specsync reads the issue's task-list state and writes it back into `tasks.md`
  before rendering, so a box ticked on GitHub sticks.

Matching tasks across sides is by normalized task text (the rendered checklist
already keys on the line). State for a task that no longer exists in `tasks.md`
is dropped with the task — no orphan reconciliation.

## Behavior

- `specsync` (normal sync) performs the reconcile: pull issue checkbox state →
  merge into `tasks.md` → push the merged result. `tasks.md` may change on disk.
- `-dry-run` reports the state delta both ways and writes nothing.
- When the same task changed wording in `tasks.md` *and* state on the issue, the
  spec's wording is kept and the issue's state is applied to it (layers are
  independent, so this is not a conflict).

## Non-goals

- Reconciling proposal prose edited on the issue — the issue description is rarely
  edited post-creation; prose stays spec-authoritative. Only task-list state flows
  back.
- A general 3-way text merge. State is a boolean per task; that is all that flows
  issue→spec.
- Cross-provider state semantics beyond GitHub task-lists — later providers
  implement a `TaskState` read; the policy is unchanged.
