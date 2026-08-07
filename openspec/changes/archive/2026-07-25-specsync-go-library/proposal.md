# Proposal: specsync Go library

> **Note on layout:** This change originally specced a `pkg/specsync/` subdirectory.
> The move was never executed — the library shipped at the module root
> (`github.com/androidand/specsync`) and that layout was adopted instead.
> The proposal below is corrected to describe what actually shipped.

## Why

specsync is currently a standalone CLI binary. Orchestrators like skein need
the same operations (sync, pull, branch naming, worktree creation) as
first-class function calls — not subprocess output to parse. Exporting an
importable `specsync` package at the module root makes specsync the shared
engine for both CLI and embedded use, with structured return values and no
shell boundary.

## What

- `specsync` package at the module root with exported API: `Sync()`, `Pull()`,
  `Scan()`, `BranchName()`, and `CreateWorktree()`
- `Sync()` returns `SyncResult{IssueNumber int, URL string, Created bool}` —
  callers get the GitHub Issue number directly, no file I/O needed
- `BranchName(issueNumber int, slug string) string` encodes the canonical
  convention: `feat/<N>-<slug>` (consistent across standalone and embedded use)
- `CreateWorktree(repoRoot, branch, path string) error` wraps
  `git worktree add -b <branch> <path>` for CLI use; embedded callers like
  skein call their own worktree manager using the returned branch name
- `cmd/specsync/main.go` is a thin wrapper: flag parsing → package calls →
  output formatting. Zero logic in main.
- `pluggable-providers` change builds on top of this: the `WorkProvider`
  interface lives in `provider.go` (root) and is part of the library API

## Scope

**In scope**
- Export the five functions listed above with stable signatures at the module root
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
  `github.com/androidand/specsync` (module root) and wires issue number into
  `ChangeStateStore` and branch naming.
  Must land after or in parallel with this change (use `go.work` to unblock).
  GitHub: https://github.com/androidand/skein/issues/20

## Risks

- **Import path break**: all existing callers of `specsync` as a binary are
  unaffected; only the Go module path for library consumers is the module root.
  No public library consumers exist today.
- **Circular test dependencies**: root-package tests stay at the root; test
  coverage must not regress.
- **Flat package**: the root package has ~20 `.go` files. No restructuring
  occurred — all files remain at the module root with `package specsync`
  declarations unchanged.
- **`pluggable-providers` sequencing**: if that change lands first in the root
  package, the integration is straightforward since the package is already flat.
