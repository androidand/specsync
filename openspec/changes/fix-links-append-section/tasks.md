# Tasks: keep appended links out of dependency sections

## Write path (`cache.go`)
- [x] `saveLinksToMD` finds the last `## ` heading in the existing file
- [x] If that heading is not `Related`, write `## Related` before the appended entries
- [x] No heading, or last heading `Related`: append as today
- [x] Existing bytes unchanged in every case (append-only invariant)

## Tests
- [x] Regression: file ending in `## Blocked by` + link → new ref parses as related, blockedBy unchanged; observed failing on `main` before the fix
- [x] Same for `## Blocks` and for an unknown author heading
- [x] File ending in `## Related`: no second heading written
- [x] Plain-list file without headings: output unchanged from today (existing `TestSaveLinksToMD_NewFile` and `_AppendsAfterUnterminatedLine`)
- [x] Repeat link of an already-recorded ref writes nothing (idempotence kept)
- [x] Full suite green; `boundary_test.go` (stdlib-only) unaffected (see Discoveries for `TestPull_CannotResolveFromBranch`)
