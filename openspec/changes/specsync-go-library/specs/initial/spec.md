# spec.md — specsync-go-library

## ADDED

- `pkg/specsync/sync.go`: exported `SyncOptions` struct and `Sync` function returning `(*SyncResult, error)`
- `pkg/specsync/worktree.go`: exported `BranchName` and `CreateWorktree` functions
- `pkg/specsync/provider.go`: `WorkProvider` interface (consumed by pluggable-providers change)
- `cmd/specsync/worktree.go`: CLI `worktree` subcommand implementation
- `pkg/specsync/*.go`: migrated from root package (all current `package specsync` files)
- `pkg/specsync/*_test.go`: migrated from root package (all current `package specsync` tests)
- `cmd/specsync/main.go`: updated to import `pkg/specsync`; zero logic remains here
- `pkg/specsync/sync_test.go`: tests for `Sync` and `SyncResult` (moved from root)
- `pkg/specsync/worktree_test.go`: tests for `BranchName` and `CreateWorktree` (moved from root)
- `pkg/specsync/provider_test.go`: tests for `WorkProvider` interface (moved from root)

## MODIFIED

- `cmd/specsync/main.go`: imports `pkg/specsync` instead of root; delegates all functionality
- `pkg/specsync/*.go`: `package specsync` declarations remain unchanged; internal imports may need updating

## REMOVED

- None from root: all `.go` files are moved into `pkg/specsync/`
