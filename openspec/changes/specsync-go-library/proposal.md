# Proposal: specsync Go library (pkg/specsync)

## Why

specsync is currently a standalone CLI binary. Orchestrators like skein need
the same operations (sync, pull, branch naming, worktree creation) as
first-class function calls — not subprocess output to parse. Extracting an
importable `pkg/specsync` package makes specsync the shared engine for both
CLI and embedded use, with structured return values and no shell boundary.

## What

- `pkg/specsync` package with exported API: `Sync()`, `Pull()`, `Scan()`,
  `BranchName()`, and `CreateWorktree()`
- `Sync()` returns `SyncResult{IssueNumber int, URL string, Created bool}` —
  callers get the GitHub Issue number directly, no file I/O needed
- `BranchName(issueNumber int, slug string) string` encodes the canonical
  convention: `feat/<N>-<slug>` (consistent across standalone and embedded use)
- `CreateWorktree(repoRoot, branch, path string) error` wraps
  `git worktree add -b <branch> <path>` for CLI use; embedded callers like
  skein call their own worktree manager using the returned branch name
- `cmd/specsync/main.go` becomes a thin wrapper: flag parsing → `pkg/specsync`
  calls → output formatting. Zero logic in main.
- `pluggable-providers` change builds on top of this: the `WorkProvider`
  interface lives in `pkg/specsync/provider.go` and is part of the library API

## Scope

**In scope**
- Reorganise all current root-package `.go` files into `pkg/specsync/`
- Export the five functions listed above with stable signatures
- `cmd/specsync/main.go` delegates entirely to the package; behaviour unchanged
- `BranchName` and `CreateWorktree` as new functionality (zero today)
- `go.work` setup instructions in README for local co-development with skein

**Not in scope**
- Switching from `gh` CLI to direct GitHub REST API calls (tracked separately;
  `pluggable-providers` owns the provider interface that enables this swap)
- Sub-issue / epic support
- Any skein-internal changes (tracked in skein's `specsync-library-integration`)

## Related

- skein: `openspec/changes/specsync-library-integration` — consumer side; imports
  `pkg/specsync` and wires issue number into `ChangeStateStore` and branch naming.
  Must land after or in parallel with this change (use `go.work` to unblock).
  GitHub: https://github.com/androidand/skein/issues/20

## Risks

- **Import path break**: all existing callers of `specsync` as a binary are
  unaffected; only the Go module path changes for library consumers. No public
  library consumers exist today.
- **Circular test dependencies**: root-package tests must move to
  `pkg/specsync/` or `cmd/specsync/`; test coverage must not regress.
- **Flat → nested migration**: the root package currently has ~20 `.go` files.
  Moving them requires updating all `package specsync` declarations and internal
  imports. No logic changes — purely structural.
- **`pluggable-providers` sequencing**: if that change lands first in the root
  package, the migration gets slightly larger. Preferred order: this change
  first, then pluggable-providers builds on `pkg/specsync`.
