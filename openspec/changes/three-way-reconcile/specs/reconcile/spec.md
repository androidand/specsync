# Reconcile improvements

The existing two-way reconcile uses monotonic union (checked wins). This spec defines the 3-way reconcile and stable task ID improvements.

## 3-way reconcile

### Requirement: Un-checks propagate from issue

When a task is checked in `tasks.md` at last sync (base), but the issue shows unchecked, the current state should reflect the issue's un-check — the human explicitly undid the work.

### Requirement: Base state stored per sync

After each sync, specsync stores a hash of `tasks.md` to enable 3-way comparison on the next reconcile.

## Stable task ID

### Requirement: Task matching survives wording changes

When a task in `tasks.md` is rewritten (e.g., clarifying language), the stable ID derived from the original text allows the reconcile engine to still match the corresponding issue checkbox and merge state.
