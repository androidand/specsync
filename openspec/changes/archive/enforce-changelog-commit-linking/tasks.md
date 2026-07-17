# Tasks: enforce changelog commit linking

## Code
- [x] Add `-fail-on-unlinked-commits` bool flag to `runChangelog` in `cmd/specsync/changelog.go`
- [x] After `BuildChangelog`, collect entries where `Hash != "" && Slug == ""`
- [x] When the flag is set and any such entries exist, print them (hash + description) and exit non-zero via `fail()`
- [x] Flag has no effect on default output — read-only detection, same as `-fail-on-archive-candidates`

## CI
- [x] Add a step to `.github/workflows/ci.yml` running
      `go run ./cmd/specsync changelog -fail-on-unlinked-commits`, next to the
      existing "Enforce OpenSpec archive hygiene" step
- [x] Confirm it runs on `pull_request` (not just push to main) so it blocks before merge

## Tests
- [x] Unit test: a trace with one loose-conventional commit (no ref) → flag detects it, error lists hash + description
- [x] Unit test: a trace with only properly-linked commits → flag is a no-op, exit 0
- [x] Unit test: a loose *non-conventional* commit (already silently omitted) does NOT trigger the flag — this only catches commits that would otherwise reach the raw fallback, not the already-silent-omission case

## Docs
- [x] AGENTS.md Dogfooding section: replace "run `specsync changelog` and read it" with the literal `-fail-on-unlinked-commits` command
- [x] Cross-link from `title-hygiene-on-pull`'s proposal (already done) confirming this is the separate, process-side half of the changelog-quality fix

## Verified manually against real history

- [x] `changelog -fail-on-unlinked-commits -since v0.7.0 -until v0.8.0` correctly flags all 12 raw-fallback commits from the actual embarrassing 0.8.0 section, exit 1
- [x] Confirmed the same problem already existed in 0.7.0 (4 unlinked commits), not just 0.8.0
- [x] Confirmed it also flags recent unreleased work (including this session's own specsync commits) — real, current, self-inflicted evidence the gate is needed, not rewritten retroactively (would require rewriting already-pushed history)
