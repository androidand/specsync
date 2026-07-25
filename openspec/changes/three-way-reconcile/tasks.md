# Tasks

## Stored base state
- [x] Add `base` field to `Ref` struct: stores SHA of `tasks.md` at last sync
- [x] On reconcile, read base tasks.md from git history; compute 3-way diff against current tasks.md and issue state
- [x] Propagate un-checks from issue when base was checked but current is unchecked (3-way merge)
- [x] Tests: 3-way merge with un-check; base state preserved across syncs

## Stable task ID
- [ ] Generate stable ID per task line (hash of original normalized text at creation time)
- [ ] Store ID in `.specsync/tasks.json` (gitignored cache) alongside ref data
- [ ] Match issue tasks to spec tasks by stable ID first, text fallback second
- [ ] Tests: wording change in spec preserves state match via stable ID

## Verification
- [ ] `go build ./...`, `go test ./...`, `gofmt` clean
