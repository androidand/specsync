# Tasks: links.md is append-only

## Append-only write (`cache.go`)
- [x] `saveLinksToMD` reads the existing file and appends only unrecorded refs
- [x] Take an `openspecDir` argument so slug entries resolve during dedup
- [x] Deduplicate on resolved `provider#id` via `parseLinksMD`, not string match
- [x] Write nothing when every ref is already recorded (byte-for-byte idempotent)
- [x] Insert a newline first when the existing file does not end in one
- [x] Keep from-scratch output a plain list — no section header imposed

## Call sites
- [x] `link.go` passes `opts.OpenSpecDir`
- [x] `pull.go` passes `opts.OpenSpecDir` (re-pull no longer flattens the file)
- [x] `spinoff.go` passes `opts.OpenSpecDir`

## Tests
- [x] Authored prose + `## Blocked by` + entries survive a save (`links_md_test.go`)
- [x] Already-recorded ref writes nothing, across shorthand / URL / in-section / with-prose
- [x] Sibling-slug entry counts as recorded (no duplicate shorthand)
- [x] New file is a bare list; no refs creates no file
- [x] Unterminated last line gets its own newline
- [x] End-to-end `Link` preserves authored content and is idempotent (`link_test.go`)
- [x] Full suite green; `boundary_test.go` (stdlib-only) unaffected
