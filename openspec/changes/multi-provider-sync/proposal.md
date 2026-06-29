# Multi-provider sync (fan-out)

specsync is tracker-agnostic, but `Sync` projects to exactly **one** provider per
run. A project that wants the same OpenSpec change visible in two places at once —
GitHub for humans and Beads for an agent task graph, say — must run specsync
twice and can never reconcile inbound state from both in one pass. Fan-out makes
"the same change, projected to every tracker you use" a single operation.

This is the tracker-agnostic principle taken to its natural end: not "pick a
tracker," but "keep all your trackers aligned with the spec at once."

## Principle: one source of truth, many spokes (a star, never a mesh)

This is the line that keeps fan-out a sync problem and not a distributed-database
problem:

- **OpenSpec is the single source of truth.** Every provider is a projection of
  it.
- **Every edge is OpenSpec↔provider.** There is never a provider↔provider edge.
- **Reconcile is N-inbound-to-1:** done-state read from any provider merges into
  `tasks.md` by monotonic union; OpenSpec owns task wording/order.

Because each provider reconciles *independently against the one authority* —
exactly as today, just looped — fan-out needs **no conflict-resolution model, no
stored base state, and no runtime**. The instant state were allowed to flow
provider→provider (a mesh), specsync would need vector clocks / 3-way merge and a
running reconciler, forfeiting the invoke-and-exit property that makes it
adoptable anywhere. That door stays shut (see Non-goals).

## What

- `Sync` accepts a **set** of providers instead of one.
- Provider-set selection via a repeatable `-provider` flag (and/or config), e.g.
  `-provider github -provider beads`; default remains `github` alone.
- Per change: reconcile inbound from every provider (union into `tasks.md`), then
  push the rendered item to every provider. `.specsync/refs.json` already keys by
  provider name, so each provider's ref coexists per change.
- Result reporting aggregates per-provider outcomes (created/updated, reconcile
  flips) per change.

## Scope

- Repeatable `-provider` flag + config resolution.
- `Sync` loop over the provider set; aggregate `Result` across providers.
- Per-provider dry-run output.
- Failure isolation: one provider erroring must not corrupt another provider's
  refs or abort the rest of the run unceremoniously.

## Non-goals

- **No second source of truth.** Providers receive wording/scope and contribute
  only done-state inbound; none becomes authoritative over OpenSpec.
- **No spoke-to-spoke sync.** State never flows provider→provider directly; it
  always passes through OpenSpec. No mesh, no CRDTs, no cross-tracker conflict
  resolution.
- **No runtime.** Still invoke-and-exit — no daemon, no watch loop, no persisted
  base state.
- Not a control plane and not a memory tool (inherited stance).
