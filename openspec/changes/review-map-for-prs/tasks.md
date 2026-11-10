# Tasks

## 1. Map

- [ ] 1.1 Resolve a branch, revision range or change request to a base and head
- [ ] 1.2 List commits in order with per-file additions and deletions
- [ ] 1.3 Categorise files from a documented path-pattern table (source, test, docs, config, generated, other)
- [ ] 1.4 Parse `## Review focus` globs and reasons from the change's `proposal.md`
- [ ] 1.5 Link commits to tasks only by an explicit task id in the message

## 2. Output

- [ ] 2.1 Markdown checklist: needs judgement first, then commits in order, then totals by category
- [ ] 2.2 `-json` with identical ordering

## 3. Equivalence check

- [ ] 3.1 `-equivalent <old> <new>`: compare trees, print identical or the differing paths; exit non-zero on difference
- [ ] 3.2 State plainly that it proves content, not history

## 4. Tests

- [ ] 4.1 Fixture branch: ordering, sizes, categories, totals
- [ ] 4.2 Focus section present and absent
- [ ] 4.3 Explicit task link versus unlinked commits
- [ ] 4.4 Equivalence: identical trees after a squash, and a differing pair

## 5. Docs

- [ ] 5.1 Implement each new command or flag once, as a `Command` type per `make-commands-discoverable`; README, `agent-help`, the overview and the skill are generated, never hand-written
- [ ] 5.2 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
