# Tasks

## Decide first

- [x] Pick `maxSlugLen`. Proposal: 48. Sanity-checked against this repo's own
      slugs (`add-agent-help-command` is 22; the longest existing ones sit
      well under 40) — 48 confirmed.
- [x] Decide whether `epic.go`'s `epic:` prefix counts toward the cap.
      Decision: no — the cap applies inside `slugify` to the generated slug
      only; `epic.go` prefixes `epic:` afterward, so the prefix is fixed
      overhead on top of an already-capped, readable slug (documented as a
      comment on `maxSlugLen`).
- [x] Check what `pull` does today when the target change directory already
      exists, and decide whether truncation needs a collision suffix or the
      existing behaviour already covers it.
      Finding: `pull.go` has **no existing collision guard at all** —
      `os.MkdirAll` + `os.WriteFile` silently overwrite `proposal.md`/
      `tasks.md` in place whenever the derived slug matches an existing
      directory, regardless of whether it's the same issue being re-pulled
      or a different one that happened to collide. This is a pre-existing
      gap independent of slug length: two different issues with literally
      identical titles already collide today, untruncated. Truncation raises
      the odds (demonstrated by `TestSlugifyDistinctLongTitlesCanCollideOnTruncation`)
      but doesn't create a new class of bug, and fixing it needs to
      distinguish "re-pull of the same issue" (intentional overwrite) from
      "different issue, same slug" (should not overwrite) — that requires
      checking the existing dir's ref cache against the issue being pulled,
      which is a separate, real fix orthogonal to capping slug length.
      **No collision suffix added here.** `spinoff.go` already errors on an
      existing child dir (unaffected either way). `epic.go`'s slug is not
      used as a directory path, so no collision surface there.

## Implement

- [x] Add the cap inside `slugify` (`pull.go:485`) so all three call sites get
      it: `pull.go:93`, `epic.go:83`, `spinoff.go:92`.
- [x] Truncate at the last hyphen at or before the cap so words stay whole.
- [x] Never emit a trailing or leading hyphen after truncation.
- [x] Hard-truncate when the first word alone exceeds the cap, rather than
      returning an empty slug — `pull` errors out on an empty slug, so this
      would turn a long single-word title into a failure.

## Test

`slugify` currently has no tests at all.

- [x] Table test: normal short title unchanged; long title truncated on a word
      boundary; title whose first word exceeds the cap; title of only
      punctuation (already returns empty — lock that in); leading/trailing
      punctuation; consecutive punctuation runs.
- [x] Regression case from the report: the 103-character portal#4331 title
      should produce something short and readable.
- [x] Collision-risk case: two distinct long titles that truncate to the same
      stem (`TestSlugifyDistinctLongTitlesCanCollideOnTruncation`) — documents
      the known, accepted risk rather than papering over it, per the "Decide
      first" finding above (no suffix logic added).
