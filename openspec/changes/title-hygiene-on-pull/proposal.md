# Title hygiene: clean titles on pull, not just on push

## Why

Agents create issues through multiple tools — `gh issue create`, the backlog MCP, specsync `pull` — and each writes whatever title it wants. When an issue is pulled into an OpenSpec change, the raw title is copied verbatim into `proposal.md` H1. No cleaning happens on the inward path.

This means messy titles from external tools silently propagate into the OpenSpec change:

- `Design: resource-select flavor of the integration fields schema (credential → list resources → multi-create)`
- `Migrate to Prisma 7 \`prisma-client\` generator (rewrite ~450 imports)`

The outward path (`sync`) already cleans titles via `shortenTitle()`, but by then the dirty title is already on disk in `proposal.md`. The OpenSpec change has the wrong title from the start.

This isn't cosmetic. `ReleaseNote(c Change)` in `changelog.go` falls back to `c.Title` verbatim whenever a proposal has no `## Release note` section — so an unclean title doesn't just make an ugly issue title, it becomes the **permanent public changelog entry** for that change (see `CHANGELOG.md`'s 0.6.0 section for what a clean entry looks like, vs. later sections). Title hygiene is one direct contributor to why this repo's own dogfooded changelog — meant to be proof the tool works — has read as an embarrassment rather than a showcase. (A second, larger contributor is commits shipping without a linked issue at all, falling into the raw `looseEntry()` fallback; that's a process-discipline problem tracked in `AGENTS.md`'s Dogfooding rules, not something a title fix alone can solve.)

## What Changes

- **Apply `shortenTitle()` on the pull path.** When `specsync pull` reads an issue title, clean it before writing into `proposal.md` H1.
- **Report when the title was cleaned.** The CLI shows `title cleaned: "before" -> "after"` so the user sees what changed.
- **`shortenTitle()` strips:** parentheticals, backtick-enclosed tool names, trailing detail words (generator, client, adapter, component, etc.).
- **The full title stays in the issue body** for documentation. The issue title and proposal H1 are now the same clean version.

## Capabilities

### New Capabilities
- `title-hygiene` — `shortenTitle()` applied on both pull (inward) and sync (outward) paths, with visible output when cleaning occurs.

## Impact

- `pull.go`: `Pull()` now calls `shortenTitle()` on the issue title before `splitBody()`. `PullResult` gains `TitleCleaned`, `TitleBefore`, `TitleAfter` fields.
- `change.go`: `shortenTitle()`, `stripParentheticals()`, `stripBackticks()`, `trimDetailWords()` — already exist from the sync-side fix.
- `cmd/specsync/main.go`: `runPull()` prints `title cleaned: "before" -> "after"` when `TitleCleaned` is true.
- `sync.go`: already applies `shortenTitle()` in `WorkItemFor()` — no change needed.
- The specsync skill file gains a note that titles are cleaned on pull, so agents don't need to write clean titles manually.
