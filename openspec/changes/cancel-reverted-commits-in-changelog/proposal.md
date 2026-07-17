# Changelog ignores commits reverted within the same release range

## Why

A commit that lands and is reverted before the next release is a net no-op — the release does not contain its behavior. But `specsync changelog` renders the original commit as a raw fallback entry (and hides the revert, whose `Revert "..."` header is not conventional). The pending 0.9.0 section claims two title-rewriting fixes that were both reverted before release:

- `clean issue titles on pull, not just on push (08c9109b)` — reverted in `da4b1dc`
- `shorten issue titles by stripping parentheticals, backticks, and detail words (95b9345)` — reverted in `20937fd`

A changelog that describes behavior the release does not ship is a correctness bug, per this repo's dogfooding rules.

## What Changes

- `ParseCommit` extracts the reverted hash from git's standard `This reverts commit <hash>.` body line into a new `Commit.RevertsHash` field.
- `BuildChangelog` cancels revert pairs before classifying commits: when both a commit and its revert are inside the release range, both are dropped from linked and loose entries alike. Pairs are matched newest-revert-first so revert-of-revert chains resolve to their true net effect.
- A revert whose target is outside the range still renders (the release really does change behavior relative to the previous one).
- Cancelled commits are also excluded from `-fail-on-unlinked-commits` — a net no-op cannot degrade the changelog.

## Release note

The changelog now ignores commits that were reverted within the same release range, so net no-op work no longer produces entries describing behavior the release doesn't contain.

## Impact

- `commit.go`: `Commit.RevertsHash`, parsed in `ParseCommit`.
- `changelog.go`: `cancelRevertPairs()` applied at the top of `BuildChangelog`.
- Tests: pair cancellation, revert-of-revert chain, out-of-range revert kept, linked-pair cancellation.
