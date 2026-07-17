# Tasks: enforce changelog commit linking

## Code
- [ ] Add `-fail-on-unlinked-commits` bool flag to `runChangelog` in `cmd/specsync/changelog.go`
- [ ] After `BuildChangelog`, collect entries where `Hash != "" && Slug == ""`
- [ ] When the flag is set and any such entries exist, print them (hash + description) and exit non-zero via `fail()`
- [ ] Flag has no effect on default output — read-only detection, same as `-fail-on-archive-candidates`

## CI
- [ ] Add a step to `.github/workflows/ci.yml` running
      `go run ./cmd/specsync changelog -fail-on-unlinked-commits`, next to the
      existing "Enforce OpenSpec archive hygiene" step
- [ ] Confirm it runs on `pull_request` (not just push to main) so it blocks before merge

## Tests
- [ ] Unit test: a trace with one loose-conventional commit (no ref) → flag detects it, error lists hash + description
- [ ] Unit test: a trace with only properly-linked commits → flag is a no-op, exit 0
- [ ] Unit test: a loose *non-conventional* commit (already silently omitted) does NOT trigger the flag — this only catches commits that would otherwise reach the raw fallback, not the already-silent-omission case

## Docs
- [ ] AGENTS.md Dogfooding section: replace "run `specsync changelog` and read it" with the literal `-fail-on-unlinked-commits` command
- [ ] Cross-link from `title-hygiene-on-pull`'s proposal (already done) confirming this is the separate, process-side half of the changelog-quality fix
