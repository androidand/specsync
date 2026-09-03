# Collapse synced issue bodies to title + tasks by default

## Why

A synced issue currently renders the full raw `proposal.md` (all of Why,
Proposed Changes, Capabilities, Non-Goals, Open Questions, Release Notes —
verbatim, redundant leading H1 included) inline, followed by Original ask,
Design notes, and Discoveries when present, before the reader ever reaches
`## Tasks`. Two real complaints trace back to this:

- A colleague reviewing synced issues wants exactly title + task checklist,
  nothing else, and doesn't see the point of the rest living in the tracker
  at all.
- GitHub's body size limit is real. `sync-design-notes` (shipped in v0.13.0)
  already designed a comment-overflow escape valve for the specific case of
  an oversized `design.md`. Nothing today makes the *common* case — a
  normal-sized change with normal-sized prose — look any shorter than a
  change that's actually pushing the limit.

Collapsing sections doesn't shrink the payload (GitHub still stores every
byte), so it doesn't replace that overflow mechanism or generation
discipline. It's a separate, real problem: what the reader sees by default.

## Proposed Changes

- **Wrap non-task sections in native `<details>`.** `WorkItemFor`
  (`sync.go`) renders `## Tasks` and `## Plan changes`/dependency sections
  visible as today. Everything else — the proposal body (H1 stripped, since
  `WorkItem.Title` already carries it), `## Original ask`, `## Design
  notes`, `## Discoveries` — is wrapped:

  ```markdown
  <details>
  <summary>Original ask</summary>
  <!-- specsync:section=original-ask -->

  {content}

  </details>
  ```

  One collapse for the whole proposal body (not one per proposal.md
  subsection) — simplest, and stays correct as proposal.md's own section
  set evolves.
- **`designNotesSection` (github.go) reuses the same wrap helper.** The
  design-notes overflow mechanism does a literal string replace to swap the
  inlined section for a stub comment link; that replace must target the
  exact same substring `WorkItemFor` renders, so `designNotesSection` calls
  the shared `wrapSection` helper instead of duplicating its format. Without
  this, wrapping Design notes would silently break the overflow check (the
  replace would never match, and an oversized body would ship unchecked).
- **HTML-comment section markers, not heading text, drive parsing.**
  `splitBody` (`pull.go`) currently matches literal `"## Original ask"` /
  `"## Design notes"` / `"## Discoveries"` lines. Add matching on
  `<!-- specsync:section=X -->` as the primary boundary, with the existing
  bare-`## ` match kept as a fallback so issues synced by an older specsync
  (not yet re-synced under this change) still pull correctly. A design-notes
  section that has overflowed to a linked comment carries no marker — its
  content lives in the comment, read via the existing
  `ReadDesignNotesComment` path, untouched by this change.
- **AGENTS.md gets a Body conventions section**, mirroring the existing
  Title conventions bad/good pair:
  - Bad: "We initially considered X, but after discussion decided Y, then
    later realized Z wouldn't work, so..."
  - Good: "Uses Y because Z."
  - Rule: Why/Design notes state the current decision, present tense.
    Narrated history belongs in `## Discoveries` (which exists for exactly
    that), not in Why or Design notes.
- **Stretch: a body-length advisory**, parallel to `title could be
  tighter` — flag (never rewrite) a Why/Design notes section that trips
  narrative markers ("initially", "we later", "after reconsidering") or
  exceeds a soft line count, printed on sync/pull.

## Capabilities

### Modified Capabilities

- `change-body-composition` (shipped by `sync-design-notes`, v0.13.0):
  extends it with collapsed rendering for Proposal/Original ask/Design
  notes/Discoveries, the section-marker format that makes collapsing
  round-trip through pull, and the redundant-H1 strip. Does not touch the
  existing section-ordering or overflow-to-comment requirements.

## Non-Goals

- **Not** per-proposal-subsection collapsing (Why/Proposed Changes/etc.
  each in their own `<details>`) — one collapse for the whole proposal
  body is enough and far less brittle.
- **Not** a second sync backend for any section — everything stays in the
  one issue body; only its default visibility changes.
- **Not** touching the comment-overflow mechanism's behavior — only the
  string it matches against, so wrapping doesn't silently break it (see
  Proposed Changes above).
- **Not** collapsed-by-default being configurable per-repo in v1 — ship one
  default (collapsed) and revisit if it's actually wrong for someone,
  rather than adding a setting nobody's asked for yet.

## Open Questions

1. Exact `<summary>` labels — "Proposal" vs. the proposal's own title
   context? Leaning "Proposal" (matches "Original ask"/"Design
   notes"/"Discoveries" as plain nouns).
2. Is the body-length advisory in scope for this change's first PR, or a
   follow-up? Leaning follow-up — AGENTS.md conventions alone likely fix
   most of it, and an advisory heuristic (narrative-marker detection) is
   worth tuning against real synced bodies before shipping.

## Release Notes

Synced issues now show title and task checklist by default; the proposal,
original ask, design notes, and discoveries are one click away in collapsed
sections instead of pushing the tasks down the page. No content is removed
or moved out of the issue.
