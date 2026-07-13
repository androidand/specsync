# Spec-driven changelog: the changelog is the spec history

## Why

A changelog derived from commits is always noise, because commits document *how
the sausage was made* — written mid-flow, for developers. What a changelog
should publish is *what was decided and shipped*, and specsync is the only tool
that holds that: every shipped change has a human-reviewed `proposal.md` with a
Why and a What, and the trace graph already binds commits → changes → issues
with evidence. Today this repo's own releases prove the problem: goreleaser's
`changelog: use: github` dumps raw `* <sha>: <subject> (@author)` lines into
the GitHub Release body, and specsync.se renders that garbage verbatim.

`add-release-traceability` explicitly deferred changelog ownership — "a future
opt-in could, if demanded." Demand has arrived. This is also the long-term
memory story: teams that keep OpenSpec changes out of git (or delete issues)
still need a durable record of what was done and why. A curated CHANGELOG.md in
git — small, human-language, AI-bloat-free — plus the closed issues *is* that
record.

## Release note

New `specsync changelog` command: a Keep a Changelog section generated from
your OpenSpec changes — one entry per shipped change in plain user language,
never a raw commit dump.

## What Changes

- Add a **`specsync changelog`** command that, for a revision range (default:
  latest tag → HEAD), emits a Keep a Changelog-formatted section:
  **one entry per shipped change, not per commit**, grouped into
  Added/Changed/Fixed/Removed/Security, with the linked issue reference.
- Entry text comes from an optional **`## Release note`** section in
  `proposal.md` — written at planning time, when the author knows why —
  falling back to the proposal title. The release note travels with the spec
  to the issue like every other section, so it is reviewed where the team
  reads it.
- The category is derived from signals specsync already computes: OpenSpec
  requirement deltas (ADDED → Added, REMOVED → Removed, MODIFIED → Changed)
  and linked conventional commits (fix → Fixed), with breaking markers
  flagged **Breaking:**.
- Commits linked to no change (the trace's loose ends) fall back to their
  conventional-commit subject; `feat`/`fix`/breaking are included, plumbing
  types (chore/docs/ci/…) are omitted from the published section and counted
  in a footer note so omission is visible, never silent.
- Output modes: human preview (default), **`-release-notes`** (bare markdown
  for `goreleaser release --release-notes`), **`-json`**, and opt-in
  **`-apply`** which prepends/replaces the version section in `CHANGELOG.md`
  idempotently.
- **Stays in its lane**: when the detected release tool owns the changelog
  (release-please, changesets, …), `-apply` refuses without `-force`;
  read-only emission is always allowed.

### Out of scope

- Bumping versions, tagging, publishing — the release tool owns those, always.
- Rewriting historical released sections in CHANGELOG.md (only the section for
  the version being prepared is managed).
- Backfilling release notes for changes shipped before this feature existed.

## Capabilities

- `changelog-generation`: build + render the spec-driven changelog.
