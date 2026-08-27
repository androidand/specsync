# Sync design.md into the issue body

## Why

`design.md` is a first-class OpenSpec artifact — its mere presence already
makes `IsSignificant` treat a change as worth a full kept archive
(`change.go:717`) — but its *content* is invisible to the tracker. `Change`
loads `original-ask.md` and `discoveries.md` and folds both into the synced
issue body (`## Original ask`, `## Discoveries`); `design.md` gets no such
treatment anywhere in `change.go` or `sync.go`. The asymmetry is undocumented
too: README's behaviors list documents Original ask and Discoveries
rendering but never mentions design.md.

This isn't theoretical. Reproduced 2026-08-27 in `ExopenGitHub/portal`: a
`bolag-legal-entity-picker` change had a substantial `design.md` (picker
pattern decision, a component-extraction plan for an ExoKit-shared molecule,
several other recorded decisions) written entirely locally. The worktree it
lived in was force-removed as part of routine cleanup after its sibling PR
merged. `openspec/` is gitignored by convention (`portal/.gitignore:32`), so
there was no git history to recover from, and `specsync pull -issue 4276`
only restored `proposal.md` + `tasks.md` — because that's all `sync` had
ever written to the issue. The design.md content was gone; it happened to be
reconstructable from an agent's conversation history that session, which is
not a recovery mechanism anyone should rely on.

The whole point of syncing a change to a tracker is that the tracker becomes
the durable, recoverable copy. For `design.md`, today it silently isn't.

## Proposed Changes

- **Load `design.md`** into a new `Change.DesignNotes` field in `LoadChange`
  (`change.go`), identical pattern to `Discoveries`/`OriginalAsk`.
- **Render `## Design notes`** in `WorkItemFor` (`sync.go`) when non-empty.
  Ordering: `Body` (proposal) → `## Original ask` (historical context) →
  `## Design notes` (decisions made while planning) → `## Discoveries`
  (findings made while implementing) → `## Tasks` → `## Plan changes` →
  dependency sections. This follows the actual authoring timeline of a
  change, oldest to newest.
- **Pull-side symmetry**: `specsync pull` should extract a `## Design notes`
  section back into local `design.md`, the same way `parseManagedSections`
  already extracts `## Original ask` and `## Discoveries` (`pull.go:245+`).
  Whether it's write-once like `original-ask.md` or overwritten every pull
  like `discoveries.md` is an open question below — they're currently
  different semantics for a reason (original ask is a historical snapshot;
  discoveries round-trip and get stripped locally after rendering), and
  design.md needs its own explicit answer, not a default borrowed from
  whichever of the two looks more convenient to implement.
- **Overflow to a comment, not truncation.** `never-fail-a-sync-silently`
  (planned, not yet implemented) already owns pre-flight size refusal and
  explicitly rules out truncating a body to fit — its stated remedy for an
  oversized change is splitting it via the epic/sub-issue machinery. That's
  the right answer when a change has genuinely outgrown itself, but a fat
  `design.md` next to a normal-sized proposal/tasks isn't that — it's one
  section, and design.md is very often the largest single contributor in
  practice (it was the largest section by far in the portal case above).
  This change adds a lighter remedy specific to that shape: when the
  rendered body would exceed the limit, post `design.md`'s content as an
  issue comment instead of inlining it, and leave a linked pointer in the
  body (`## Design notes\n\n_Too large to inline — see [design notes
  comment](url)._`). The comment needs its own identity marker
  (`<!-- specsync:change=<slug>:design -->`), the same idempotent-upsert
  pattern the issue body itself uses for the `<!-- specsync:change=<slug>
  -->` marker, so re-syncing updates the same comment rather than piling up
  duplicates.
- **Document it**: add a bullet to README's behaviors list (next to
  Discoveries/Original ask), and update `site/features.json` per AGENTS.md's
  dogfooding rule — this ships with the change, not after it.

## Capabilities

### New Capabilities

- `change-body-composition`: what sections a synced issue body contains, in
  what order, and how an oversized section overflows to a linked comment
  instead of being dropped or truncated. This currently exists only as
  implicit behavior in `WorkItemFor` plus a few README bullets; formalizing
  it gives contributors something to check new sections against instead of
  reverse-engineering `sync.go`.

## Non-Goals

- **Not** reimplementing pre-flight size refusal — that's
  `never-fail-a-sync-silently`'s scope. This change's comment-overflow path
  is a second remedy alongside that change's "split the change" remedy, not
  a replacement for the refusal-and-report behavior itself. Ships fine
  either before or after that change lands, though the two are more useful
  together: the size check is what actually detects and messages the
  oversized-body case in the first place.
- **Not** automatic change splitting.
- **Not** syncing any other local-only file (`links.md`, `.specsync/`
  caches, `specs/` deltas). Scope is exactly design.md parity with the
  existing original-ask.md/discoveries.md treatment, plus the overflow path
  that treatment needs to be safe at scale.

## Open Questions

1. Pull semantics: is `## Design notes` write-once (like `original-ask.md`)
   or overwritten every pull (like `discoveries.md`)? A user may have
   written local design.md content richer than what made it into the issue
   (e.g. mid-edit, not yet synced) — overwriting it on pull would lose that.
   Leaning write-once/merge-don't-clobber, but needs a real decision and a
   test that pins it.
2. Comment-overflow: on by default, or behind a flag (e.g.
   `-large-sections-as-comments`)? Default-on is less surprising for the
   common case this is meant to fix, but changes sync's behavior for anyone
   already near the size limit without them asking for it.
3. When local design.md later shrinks enough to fit inline again, does sync
   move it back into the body and mark the old comment stale ("moved into
   the issue body"), or leave the comment as-is indefinitely? Leaning
   toward marking it stale rather than deleting — comments are part of the
   audit trail.
4. Does `-dry-run` need to preview the comment write too, the way it already
   previews body/marker writes? (Almost certainly yes — just naming it so
   it isn't missed.)

## Release Notes

`design.md` now syncs into the issue body as `## Design notes`, with
automatic overflow to a linked issue comment when the combined body would
exceed GitHub's size limit — no more silently-unsynced design decisions.
