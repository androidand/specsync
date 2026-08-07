# Tasks: specsync Go library (root package)

> **Note:** This change originally specced a move to `pkg/specsync/`.
> The move was never done; the package has always lived at the module root
> (`github.com/androidand/specsync`). The archived record below is corrected
> to reflect what shipped. The `pkg/` layout was intentionally abandoned —
> root is a legitimate Go layout and consumers already build against it.
> Relocating now would break them to satisfy a task list that was wrong
> from the start.

## Slice 1: Package at the module root (was: Create pkg/specsync package skeleton)

- [x] Package lives at the module root — no `pkg/` directory
  - File: `*.go` at module root (all `package specsync` files)
  - Validation: `go build .` compiles; `go build ./cmd/specsync/` compiles
- [x] `cmd/specsync/main.go` imports the root package (no `pkg/` prefix)
  - File: `cmd/specsync/main.go`
  - Validation: `./specsync --help` output unchanged; all existing CLI flags work
- [x] All `*_test.go` files remain at the module root
  - File: `*_test.go` at module root
  - Validation: `go test .` passes with same coverage as before

## Slice 2: Export Sync() with structured return value
- [x] `Sync()` returns `(Result, error)` — callers get `Result{URL string, Created bool}`
  - File: `sync.go`
  - Validation: `go test . -run TestSync` passes
- [x] `Options` struct covers all current flags (`Slug`, `DryRun`, `Reconcile`, `Repo`, `OpenspecDir`)
  - File: `sync.go`
  - Validation: `go vet .` clean; all fields documented

## Slice 3: BranchName() and CreateWorktree()
- [x] `BranchName(issueNumber int, slug string) string`
  - File: `repo.go`
  - Validation: `go test . -run TestBranchName` covers zero-issue fallback (`feat/0-change` or `feat/<slug>`)
- [x] `CreateWorktree(repoRoot, branch, path string) error` wrapping `git worktree add -b <branch> <path>`
  - File: `repo.go`
  - Validation: `go test . -run TestCreateWorktree` creates and removes a real worktree in a temp repo

## Slice 4: CLI worktree subcommand
- [x] `specsync worktree -change <slug>` subcommand: reads `.specsync/` for issue number, calls `BranchName` + `CreateWorktree`
  - File: `cmd/specsync/main.go`
  - Validation: `specsync worktree -change test-change -dry-run` prints branch name and worktree path without creating anything

## Slice 5: go.work and README
- [x] Add `go.work` example to README showing local co-development setup with skein
  - File: `README.md`
  - Validation: instructions are runnable (`go work init`, `go work use ./specsync ../skein`)
