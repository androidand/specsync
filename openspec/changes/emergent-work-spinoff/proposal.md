# Emergent work spin-off

Agentic work constantly *spawns* work that does not belong in the change being
worked on: an unrelated bug surfaces, a follow-up PR is needed, or the fix
requires touching another repo. Two bad things happen today. Either the
discovery is stuffed into the current change — scope creep, a review-hostile PR,
the original intent buried — or it is dropped because stopping to file it breaks
momentum.

specsync already has the machinery to do this cleanly: `-repo` for cross-repo
issues, `link` for relations, and (planned) typed links. What is missing is a
single move that turns a discovery into a *scoped, linked sibling* instead of
scope creep.

## Solution

**`specsync spinoff`** — spawn a new linked change from a discovery, keeping the
parent scoped:

```
# from a task line in the current change
specsync spinoff -from <slug> -task 4 -kind bug
# or from free text
specsync spinoff -from <slug> "rate limiter drops bursts under 50ms" -kind followup
# cross-repo spawn reuses existing machinery
specsync spinoff -from <slug> -repo owner/other "shared client needs a retry" 
```

It:

1. Scaffolds a new change folder — `proposal.md` seeded with the discovery text
   and a provenance line ("spun off from `<slug>` / issue #N"), plus an empty
   `tasks.md`.
2. Marks the originating task in the parent as **moved** (see `living-plan`) so
   the parent's plan stays honest and scoped.
3. Records a typed link parent↔child (`spawned-from` / `blocks` / `relates`,
   from `beads-memory-bridge`), so the next sync renders cross-references both
   ways.
4. `-kind bug|followup|task` sets a label on the projected issue.

The parent PR stays focused; the discovery survives as its own reviewable unit
with enough context to pick up cold.

## Non-goals

- Deciding *whether* something is out of scope — the human or agent decides;
  spinoff just makes acting on that decision a one-liner.
- Running or scheduling the spun-off work.
- Auto-detecting related bugs. Capture of raw discoveries lives in `living-plan`
  (`## Discoveries`); spinoff is the triage action that promotes one out.
