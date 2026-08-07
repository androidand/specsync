# spec.md — specsync-go-library

## ADDED

- `sync.go`: exported `Options` struct and `Sync` function returning `(Result, error)`
- `repo.go`: exported `BranchName` and `CreateWorktree` functions
- `provider.go`: `WorkProvider` interface (consumed by pluggable-providers change)
- `cmd/specsync/main.go`: CLI `worktree` subcommand implementation
- `*.go` at module root: all current `package specsync` files remain at root
- `*_test.go` at module root: all current `package specsync` tests remain at root
- `cmd/specsync/main.go`: imports root package; delegates all functionality

## MODIFIED

- `cmd/specsync/main.go`: imports root package (no `pkg/` prefix); delegates all functionality
- `*.go` at module root: `package specsync` declarations unchanged

## REMOVED

- None: the package has always lived at the module root
