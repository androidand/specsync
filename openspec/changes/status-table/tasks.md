# Tasks

## 1. Data

- [x] 1.1 Read requested reviewers and each reviewer's latest review state into the change request (`reviewRequests`, `latestReviews`), in the JSON and the request cache
- [x] 1.2 Show the change title as "What", falling back to the pull request title for a branch with no change

## 2. Table

- [x] 2.1 One renderer that turns rows into the table, with the check, review and size wording above, wrapping long text to the terminal width
- [x] 2.2 `status -table`: current work plus its related issues and pull requests
- [x] 2.3 `overview -table`, under the same filters as `overview`
- [x] 2.4 A row without a pull request shows `not opened` and its local state

- [x] 2.5 Link issue and request numbers: terminal hyperlinks on a terminal, `-urls` for a plain list

## 3. Tests

- [x] 3.1 Golden table for: approved, review requested, changes requested, failing checks, pending checks, no checks, no pull request, merged
- [x] 3.2 Several issues on one pull request; the repository is named when rows span repositories
- [x] 3.4 Hyperlinks add no width and appear only when asked; `-urls` lists issue and request pages; a number inside a longer one is not linked
- [x] 3.3 The new fields round-trip through JSON; an old cache entry without them still loads

## 4. Docs

- [x] 4.1 Add the flag as part of the existing commands per `make-commands-discoverable`; README, `agent-help` and the skill are generated
- [x] 4.2 Update `site/features.json`
- [x] 4.3 Create the issue, reference it in the commits, run `specsync changelog -fail-on-unlinked-commits`
