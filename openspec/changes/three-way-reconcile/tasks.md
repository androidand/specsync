# Tasks

## Stored base state
- [x] Add `base` field to `Ref` struct: stores SHA of `tasks.md` at last sync
- [x] On reconcile, read base tasks.md from git history; compute 3-way diff against current tasks.md and issue state
- [x] Propagate un-checks from issue when base was checked but current is unchecked (3-way merge)
- [x] Tests: 3-way merge with un-check; base state preserved across syncs

## Stable task ID
- [x] Match base tasks to current tasks by text then position to detect wording changes
- [x] Build reverse mapping (current text → base text) for rewritten task detection
- [x] Match issue tasks to spec tasks by base text via mapping, text fallback second
- [x] Tests: wording change in spec preserves state match via position-based mapping

## Verification
- [x] `go build ./...`, `go test ./...`, `gofmt` clean
