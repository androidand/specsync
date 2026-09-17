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
      Finding: `pull.go` had **no existing collision guard at all** —
      `os.MkdirAll` + `os.WriteFile` silently overwrote `proposal.md`/
      `tasks.md` in place whenever the derived slug matched an existing
      directory, regardless of whether it was the same issue being re-pulled
      or a different one that happened to collide. This was a pre-existing
      gap independent of slug length: two different issues with literally
      identical titles already collided, untruncated. Truncation raises the
      odds (demonstrated by `TestSlugifyDistinctLongTitlesCanCollideOnTruncation`)
      but doesn't create a new class of bug on its own — it just makes an
      existing one more likely, so it's fixed here rather than deferred.
      **Fixed** in `pull.go`: a directory that already exists is only ever
      overwritten when it's an intentional re-pull of the *same* issue (its
      ref cache names that issue), or an explicit `-change` into a directory
      with no ref cache at all (a hand-authored, never-synced local change —
      the human named that exact directory; this is the normal
      spec-first-then-pull flow, see `TestPullRecordsTaskBase`). Every other
      case — a ref cache naming a *different* issue, or an auto-derived slug
      colliding with an untracked hand-authored directory — gets a numeric
      suffix (`-2`, `-3`, ...) via `nextAvailableSlug`, or an explicit error
      for an explicit `-change` conflicting with a different tracked issue
      (pass a different `-change`, or use `specsync adopt`). `spinoff.go`'s
      own pre-existing error-on-collision is unaffected. `epic.go`'s slug is
      not used as a directory path, so no collision surface there.

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
      that `slugify` itself doesn't dedupe (it's a pure string function); the
      directory-level collision this enables is what `pull_collision_test.go`
      now covers directly (same-issue re-pull overwrites in place;
      different-issue collision on a generated slug gets suffixed;
      explicit `-change` collision with a different tracked issue errors;
      explicit `-change` collision with an untracked hand-authored directory
      still overwrites; an auto-derived slug colliding with an untracked
      hand-authored directory gets suffixed; `-dry-run` previews the
      suffixed slug without writing; `nextAvailableSlug` skips multiple
      taken suffixes).
