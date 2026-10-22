# Tasks: appended links land in the last section of links.md

## Write path (`cache.go`)
- [ ] `saveLinksToMD` finds the last `## ` heading in the existing file
- [ ] If that heading is not `Related`, write `## Related` before the appended entries
- [ ] No heading, or last heading `Related`: append as today
- [ ] Existing bytes unchanged in every case (append-only invariant)

## Tests
- [ ] Regression: file ending in `## Blocked by` + link → new ref parses as related, blockedBy unchanged; observed failing on `main` before the fix
- [ ] Same for `## Blocks` and for an unknown author heading
- [ ] File ending in `## Related`: no second heading written
- [ ] Plain-list file without headings: output unchanged from today
- [ ] Repeat link of an already-recorded ref writes nothing (idempotence kept)
- [ ] Full suite green; `boundary_test.go` (stdlib-only) unaffected
