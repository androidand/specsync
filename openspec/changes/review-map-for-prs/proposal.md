# Generate a review map for a pull request

## Why

A reviewer facing a large change has no map. The diff view lists files alphabetically,
the commit list is a wall, and nothing says which parts need human judgement and which
are mechanical. In a real case the person responsible for handing over a multi-repo change
could not find a systematic way to review it, and so could not hand it over with
confidence.

specsync already knows the pieces a map is made of. The change's proposal says what the
change is for, its tasks say what was done, the commits say in what order, and git says
which files each commit touched and how much. It does not combine them into the one
artefact a reviewer needs: *in what order to read this, what deserves attention, and
what can be left to the tests*.

A related need came up when a branch's history was rewritten into fewer commits before
review. The only way to be sure nothing changed was to compare the trees by hand.

## What Changes

- **`specsync review-map`** takes a branch, a revision range or a change request and
  prints a checklist:
  1. **Needs human judgement**: files matching the change's declared review focus.
  2. **Everything else in commit order**, each commit with its files, lines added and
     removed, and a category (source, test, docs, config, generated, other) from built-in
     language-neutral path rules.
  3. **Totals** by category, so the mechanical bulk is visible as a count, not a wall.
- **Review focus is authored, not guessed.** A change's `proposal.md` may contain a
  `## Review focus` section listing path globs and a reason each. Only those paths are
  marked as needing judgement. Without the section the map still orders and categorises,
  and marks nothing.
- **Task links.** A commit that names a task id from the change (for example `task 2.1`)
  is shown under that task; others are shown as unlinked. Nothing is matched by
  similarity.
- **Formats.** Markdown with checkboxes (paste into a description or work through in an
  editor) and `-json` for any agent.
- **`review-map -equivalent <old> <new>`** proves a rewritten branch is the same
  content: it compares the two trees and prints OK or the list of differing paths. It
  does not claim the history is equivalent, only the result.

## Generality

- Needs only git. A change request is optional input, resolved through the provider when
  a URL or number is given.
- Categories use plain path patterns (test directories and file suffixes, docs,
  lockfiles, config) that work across languages; teams extend them through the focus
  section, not through code.
- Output is a document, so it works in any editor and for any agent harness.

## Non-goals

- Not an automated reviewer: no analysis of code, no comments, no verdicts.
- Does not mark files "safe". It says only what the author declared as needing judgement
  and what the path rules call mechanical.
- Does not post to a tracker or a change request.

## Impact

- A read-only command over git history and the change's markdown; a small category rule
  table; a tree-compare helper.
- Docs: README, `agent-help`, the skill, and `site/features.json`.

## Release note

`specsync review-map` turns a branch into a review checklist: what to read first, what is
mechanical, and which commit introduced what, plus a check that a rewritten branch holds
the same content.
