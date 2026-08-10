# Tasks

## 1. State

- [x] 1.1 Persist last-written board status (option id per change) in the
      gitignored `.specsync/` state
- [x] 1.2 Treat "specsync-managed" as "unchanged since specsync last wrote",
      replacing the managed-names approximation

## 2. Reconcile

- [x] 2.1 Inbound board-status read before outbound projection
- [x] 2.2 Human move to Done-like option with incomplete tasks → surface,
      don't drag back
- [x] 2.3 Human move to active option on a complete change → reopen signal,
      aligned with `-close-completed` reopen semantics
- [x] 2.4 Precedence rules with issue open/closed lifecycle

## 3. Verification

- [x] 3.1 Faked-board tests: fresh card, untouched specsync status, human
      forward move, human backward move, reopened issue
