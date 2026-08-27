# Tasks

- [ ] 1. Add `Change.DesignNotes` field; load `design.md` in `LoadChange`
      (`change.go`), same pattern as `Discoveries`.
      Validation: a fixture change with a `design.md` loads its content into
      `Change.DesignNotes`; a change without one loads `""`.

- [ ] 2. Render `## Design notes` in `WorkItemFor` (`sync.go`) when
      non-empty, positioned between `## Original ask` and `## Discoveries`
      per the ordering in proposal.md.
      Validation: a fixture with all three sections renders them in the
      documented order; a change with no design.md renders no such section.

- [ ] 3. Decide and record the pull-side semantics (Open Question 1):
      write-once vs. overwrite-every-pull. Implement `parseManagedSections`
      handling for `## Design notes` in `pull.go` to match.
      Validation: a test pins the chosen semantics — a pull that would
      clobber richer local content either doesn't, or the decision to allow
      it is explicit and tested, not accidental.

- [ ] 4. Decide comment-overflow default (Open Question 2): on by default or
      behind a flag. Implement: when the rendered body exceeds GitHub's
      limit and `design.md` is present, post/update a marked comment
      (`<!-- specsync:change=<slug>:design -->`) instead of inlining, leave
      a linked stub in the body's `## Design notes` section.
      Validation: an oversized-body fixture with a design.md syncs
      successfully (comment created, body stays under the limit); re-syncing
      updates the same comment rather than creating a second one.

- [ ] 5. Decide and implement the shrink-back-below-limit case (Open
      Question 3): body now fits, design.md moves back inline, old comment
      marked stale rather than deleted.
      Validation: a fixture that starts oversized then shrinks moves
      design.md back into the body and edits the comment to note it moved.

- [ ] 6. `-dry-run` previews the comment write the same way it already
      previews body/marker writes.
      Validation: `-dry-run` output for an oversized-with-design.md fixture
      shows the comment it would create/update, with no network call made.

- [ ] 7. New capability spec `openspec/specs/change-body-composition/spec.md`
      with Requirement/Scenario blocks covering: section set, ordering,
      omission when empty, and the overflow-to-comment behavior.

- [ ] 8. Document in README's behaviors list (next to the existing
      Discoveries/Original ask bullets) and update `site/features.json` per
      AGENTS.md's dogfooding rule.

- [ ] 9. Regression test end to end: a change with a large design.md that
      pushes the body over the limit syncs cleanly (comment path), a normal
      small design.md syncs inline, and a pull after either recovers
      design.md locally.
