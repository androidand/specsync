# Tasks

## 1. Map

- [x] 1.1 Resolve a branch, revision range or change request to a base and head
- [x] 1.2 List commits in order with per-file additions and deletions
- [x] 1.3 Categorise files from a documented path-pattern table (source, test, docs, config, generated, other)
- [x] 1.4 Parse `## Review focus` globs and reasons from the change's `proposal.md`
- [x] 1.5 Link commits to tasks only by an explicit task id in the message

## 2. Output

- [x] 2.1 Markdown checklist: needs judgement first, then commits in order, then totals by category
- [x] 2.2 `-json` with identical ordering

## 3. Equivalence check

- [x] 3.1 `-equivalent <old> <new>`: compare trees, print identical or the differing paths; exit non-zero on difference
- [x] 3.2 State plainly that it proves content, not history

## 4. Tests

- [x] 4.1 Fixture branch: ordering, sizes, categories, totals
- [x] 4.2 Focus section present and absent
- [x] 4.3 Explicit task link versus unlinked commits
- [x] 4.4 Equivalence: identical trees after a squash, and a differing pair

## 5. Docs

- [x] 5.1 Implement each new command or flag once, as a `Command` type per `make-commands-discoverable`; README, `agent-help`, the overview and the skill are generated, never hand-written
- [x] 5.2 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
