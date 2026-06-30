# Tasks: specsync Go library (pkg/specsync)

## Slice 1: Create pkg/specsync package skeleton

- [ ] Create `pkg/specsync/` directory and move all root `.go` files into it
  - File: `pkg/specsync/*.go` (migrate from root)
  - Validation: `go build ./pkg/specsync/` compiles; `go build ./cmd/specsync/` compiles
- [ ] Update `cmd/specsync/main.go` to import `pkg/specsync` instead of root
  - File: `cmd/specsync/main.go`
  - Validation: `./specsync --help` output unchanged; all existing CLI flags work
- [ ] Move all `*_test.go` files to `pkg/specsync/`
  - File: `pkg/specsync/*_test.go`
  - Validation: `go test ./pkg/specsync/` passes with same coverage as before

## Slice 2: Export Sync() with structured return value

- [ ] Add `SyncResult` type and update `Sync()` signature to return `(*SyncResult, error)`
  - File: `pkg/specsync/sync.go`
  - Validation: `go test ./pkg/specsync/ -run TestSync` passes; `SyncResult.IssueNumber > 0` for a real repo
- [ ] Expose `SyncOptions` struct covering all current flags (`Slug`, `DryRun`, `Reconcile`, `Repo`, `OpenspecDir`)
  - File: `pkg/specsync/sync.go`
  - Validation: `go vet ./pkg/specsync/` clean; all fields documented

## Slice 3: BranchName() and CreateWorktree()

- [ ] Add `BranchName(issueNumber int, slug string) string` → `feat/<N>-<slug>`
  - File: `pkg/specsync/worktree.go`
  - Validation: `go test ./pkg/specsync/ -run TestBranchName` covers zero-issue fallback (`feat/0-slug` or `feat/<slug>`)
- [ ] Add `CreateWorktree(repoRoot, branch, path string) error` wrapping `git worktree add -b <branch> <path>`
  - File: `pkg/specsync/worktree.go`
  - Validation: `go test ./pkg/specsync/ -run TestCreateWorktree` creates and removes a real worktree in a temp repo

## Slice 4: CLI worktree subcommand

- [ ] Add `specsync worktree -slug <slug>` subcommand: reads `.specsync/` for issue number, calls `BranchName` + `CreateWorktree`
  - File: `cmd/specsync/main.go`
  - Validation: `specsync worktree -slug test-slug -dry-run` prints branch name and worktree path without creating anything

## Slice 5: go.work and README

- [ ] Add `go.work` example to README showing local co-development setup with skein
  - File: `README.md`
  - Validation: instructions are runnable (`go work init`, `go work use ./specsync ../skein`)
