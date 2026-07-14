# Fix completion lifecycle

The new `stage:complete` feature derives stage before inbound checkbox
reconciliation. A final checkbox completed in GitHub therefore needs two syncs
before the issue becomes complete or closes. A later unchecked task changes the
label back to active but does not reopen an issue previously closed by
`-close-completed`.

Make one sync produce a coherent projection: reconcile task state, derive the
resulting lifecycle stage, then update both label and open/closed state. Apply
the same lifecycle semantics through the provider abstraction for GitHub and
Beads.

## Decisions

- Recompute derived stage after reconciliation and before rendering.
- `.status` remains the explicit stage override.
- `-close-completed` makes tracker state follow the derived lifecycle: complete
  closes; returning to active reopens.
- Archived changes remain closed regardless of the flag.
- Without `-close-completed`, completion changes the stage label but does not
  close an open tracker item.

## Non-goals

- Changing monotonic checkbox reconciliation or supporting issue-side uncheck.
- Adding new lifecycle stages.
- Broad provider refactoring.
