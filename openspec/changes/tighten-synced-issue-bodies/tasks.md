# Tasks

- [x] 1. Strip the redundant leading H1 from the proposal body before
      rendering it (`WorkItemFor`, `sync.go`) — `WorkItem.Title` already
      carries it as the issue title.
      Validation: a fixture whose proposal.md starts with `# Foo` renders a
      body whose proposal section has no leading `# Foo` line.

- [x] 2. Wrap the proposal body, `## Original ask`, `## Design notes`, and
      `## Discoveries` in `<details><summary>{Label}</summary>` with a
      `<!-- specsync:section={id} -->` marker inside, per proposal.md.
      `## Tasks`, `## Plan changes`, and dependency sections stay visible,
      unwrapped. `github.go`'s `designNotesSection` (used by the existing
      comment-overflow check) calls the same wrap helper instead of
      duplicating its format, so wrapping doesn't silently break that check.
      Validation: a fixture with all sections present renders Tasks/Plan
      changes/dependencies outside any `<details>`, and Proposal/Original
      ask/Design notes/Discoveries each inside one with the correct marker;
      the existing design-notes-overflow tests still pass unmodified in
      behavior (only their fixture bodies updated to the wrapped format).

- [x] 3. Update `splitBody` (`pull.go`) to locate section content by
      `<!-- specsync:section=X -->` first, falling back to the legacy bare
      `## X` heading match for bodies not yet re-synced under this format.
      Validation: a fixture using the new marker format pulls identically
      to one using the legacy bare-heading format; round-trip (sync then
      pull) recovers original-ask.md/design.md/discoveries.md content
      unchanged, with no leaked `<details>`/marker text in the recovered
      files.

- [x] 4. Extend `openspec/specs/change-body-composition/spec.md` (already
      shipped by `sync-design-notes`, v0.13.0) with Requirement/Scenario
      blocks for collapsed rendering, the section-marker format, and legacy
      fallback — without touching its existing section-ordering or
      overflow-to-comment requirements.

- [x] 5. AGENTS.md: add a "Body conventions" section next to Title
      conventions, with the bad/good example pair from proposal.md and the
      explicit rule that narrated history belongs in Discoveries, not
      Why/Design notes.

- [x] 6. Document in README's behaviors list and `site/features.json` per
      AGENTS.md's dogfooding rule.

- [x] 7. Regression test: a change with Original ask + Design notes +
      Discoveries + a large proposal syncs with all four collapsed and Tasks
      visible; pulling it back recovers all local files unchanged;
      re-syncing an already-collapsed issue doesn't double-wrap or duplicate
      markers; the design-notes overflow-to-comment path still triggers
      correctly against the new wrapped format.
