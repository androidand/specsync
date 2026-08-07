# spec.md — specsync-go-library

> **Note on layout:** This change originally specced a `pkg/specsync/` subdirectory.
> The move was never executed — all `.go` files remain at the module root.
> The entries below are corrected to describe what actually shipped.

## ADDED

- `sync.go` (root): exported `SyncOptions` struct and `Sync` function returning `(*SyncResult, error)`
- `worktree.go` (root): exported `BranchName` and `CreateWorktree` functions
- `provider.go` (root): `WorkProvider` interface (consumed by pluggable-providers change)
- `cmd/specsync/worktree.go`: CLI `worktree` subcommand implementation
- `cmd/specsync/main.go`: delegates all functionality to root `specsync` package
- `sync_test.go` (root): tests for `Sync` and `SyncResult`
- `worktree_test.go` (root): tests for `BranchName` and `CreateWorktree`
- `provider_test.go` (root): tests for `WorkProvider` interface

## MODIFIED

- `cmd/specsync/main.go`: imports root `specsync` package; delegates all functionality

## REMOVED

- None from root: all `.go` files remain at the module root
