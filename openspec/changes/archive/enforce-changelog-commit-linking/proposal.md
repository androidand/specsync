# Fail CI when a commit ships without a linked issue

## Why

`specsync changelog` binds each commit to an OpenSpec change only through a
small set of recognized reference forms (`extractRefs` in `commit.go`): a
trailing `(#N)` in the commit header, `Closes #N`/`Fixes #N` keywords,
`Refs:`/`See-also:` trailers, or a cross-repo `owner/repo#N`. A bare `#N`
mentioned in prose is deliberately *not* counted as linking evidence.

A commit that matches none of these forms becomes "loose." If it's at least
conventional-commit-formatted, `looseEntry()` renders it as a raw
`<description> (<hash>)` line in the changelog — the "honest fallback" the
tool is designed to show rather than hide. If it isn't even conventional, it's
silently dropped into an "N internal commits omitted" counter.

This is exactly what has been degrading this repo's own changelog. Compare
`CHANGELOG.md`'s 0.6.0 section (every entry has a proper `(#37)`/`(#35)`
reference, reads as authored prose) to 0.8.0 (raw titles + bare hashes, reads
like an unfiltered `git log`). The failure mode is quiet: an agent can plan
the change properly, get it approved, sync it to an issue, and write a clean
conventional commit — and still degrade the changelog, purely by forgetting
to put `(#N)` in the commit message. Nothing today catches this before it's
published in a release.

Title hygiene (`openspec/changes/title-hygiene-on-pull/`) fixes a related but
distinct problem — an unclean *title* on a properly-linked change. This fixes
commits that aren't linked to a change at all.

## What Changes

- **`specsync changelog -fail-on-unlinked-commits`**: after building the
  changelog, check for any `ChangelogEntry` with `Hash != "" && Slug == ""`
  (a loose-but-conventional commit that reached the raw fallback). Exit
  non-zero and list the offending commits (hash + description) when set.
  Mirrors the existing `release-plan -fail-on-archive-candidates` pattern —
  same shape, same repo, no new mechanism invented.
- **Wire it into `ci.yml`** as its own step, next to the existing "Enforce
  OpenSpec archive hygiene" step, running on every PR — catches this at
  review time, not at release time when the damage is already published.
- **Not a local git hook.** specsync's own stated principle is
  invoke-and-exit, no daemon; a commit-msg hook needs local installation per
  contributor and is exactly the kind of soft convention that gets skipped.
  A CI gate applies uniformly regardless of who or what is committing.
- **Make AGENTS.md's existing rule literal.** "Must run `specsync changelog`
  before completing a change" currently means "read the output and judge it
  yourself" — point it at this flag instead, so there's a hard pass/fail, not
  a vibe check.

## Capabilities

### New Capabilities
- `changelog-commit-linking-gate` — `specsync changelog -fail-on-unlinked-commits`
  detects loose-but-conventional commits and fails; CI runs it on every PR.

## Impact

- `cmd/specsync/changelog.go`: `runChangelog` gains a `-fail-on-unlinked-commits`
  bool flag; after `BuildChangelog`, scan `cl.Entries` for `Hash != "" &&
  Slug == ""` and `fail()` listing them when the flag is set and any are found.
- `.github/workflows/ci.yml`: new step running
  `go run ./cmd/specsync changelog -fail-on-unlinked-commits` alongside the
  existing archive-hygiene step.
- `AGENTS.md`: update the Dogfooding section to name this flag explicitly
  instead of "read the output."

## Non-goals

- No local git commit-msg hook.
- No change to which reference forms `extractRefs` recognizes — the parsing
  is already deliberate; the gap is that nothing enforces using it.
- No retroactive rewrite of past unlinked commits or already-published
  `CHANGELOG.md` sections.
