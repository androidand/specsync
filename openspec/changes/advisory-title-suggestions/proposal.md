# Advisory title suggestions: warn on unwieldy titles, never rewrite them

## Why

Agents write full scope into titles — `proposal.md` H1s locally, issue titles via `gh` or the backlog MCP externally. Those verbose titles become issue titles, board cards, and (via `ReleaseNote()`'s fallback) permanent changelog entries:

- `Design: multi-select flavor of the export fields schema (load → list fields → multi-create)`
- `Migrate to Postgres 17 \`pgx/v6\` driver (rewrite ~450 call sites)`

Two earlier attempts rewrote titles automatically and both were reverted:

1. **Clean on sync** (`95b9345`, reverted in `20937fd`) — by sync time the dirty title is already the H1 on disk, and rewriting outward makes the spec and the issue disagree about the one thing they must agree on.
2. **Clean on pull** (`08c9109`, reverted in `da4b1dc`) — rewriting someone else's issue title into the proposal, even visibly, is still specsync editing author content with a lossy heuristic. The original transform also wasn't idempotent (a word-blacklist pass ate one trailing word per application, so pull/sync round-trips eroded titles) and could truncate on unbalanced backticks or clean a title down to nothing.

The conclusion both reverts point at: **a title is the author's content, and specsync's job is projection, not editing.** The same principle already governs board reconciliation, which refuses to clobber a human's card move.

## What Changes

- **specsync never rewrites a title, in either direction.** Pull writes the issue title verbatim as the proposal H1; sync pushes the H1 verbatim as the tracker title.
- **Both directions surface a suggestion instead.** When `shortenTitle()` would tighten a title, `sync` and `pull` print `title could be tighter: "..." — edit the proposal.md H1 if you agree`. `ItemResult.TitleSuggestion` and `PullResult.TitleSuggestion` carry it for programmatic callers. The decision stays with the author; fixing the H1 at the source then flows everywhere on the next sync.
- **`shortenTitle()` is conservative and safe by construction:** strips parenthetical asides only when parens are balanced (unbalanced input is left alone), removes backtick characters while keeping their content (tracker titles don't render markdown), collapses whitespace, trims trailing punctuation. No word blacklist — which words in a human's sentence are disposable is not a heuristic's call. A title that would clean down to nothing is returned unchanged. The transform is a fixpoint (applying it twice equals once), pinned by test.

## Capabilities

### New Capabilities
- `title-hygiene` — advisory title suggestions on both sync (outward) and pull (inward); no automatic rewriting anywhere.

## Impact

- `change.go`: `shortenTitle()` / `stripParentheticals()` — balanced-paren guard, backtick-marker removal, empty-result guard, fixpoint.
- `sync.go`: `ItemResult.TitleSuggestion`, set when the pushed H1 could be tighter; `WorkItem.Title` is always the H1 verbatim.
- `pull.go`: `PullResult.TitleSuggestion`, set when the pulled issue title could be tighter; the H1 is the issue title verbatim.
- `cmd/specsync/main.go`: `runSync()` and `runPull()` print the suggestion (dry-run and real mode).
- `skills/specsync/SKILL.md` (+ mirrors): documents the warn-never-rewrite contract and the "H1 = WHAT, not HOW" convention.
