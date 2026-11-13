# Tasks

## 1. Data

- [x] 1.1 Read requested reviewers and each reviewer's latest review state into the change request (`reviewRequests`, `latestReviews`), in the JSON and the request cache
- [x] 1.2 Show the change title as "What", falling back to the pull request title for a branch with no change

## 2. Table

- [x] 2.1 One renderer that turns rows into the table, with the check, review and size wording above, wrapping long text to the terminal width
- [x] 2.2 `status -table`: current work plus its related issues and pull requests
- [x] 2.3 `overview -table`, under the same filters as `overview`; it warns when pull requests were not looked up
- [x] 2.4 A row without a pull request shows `not opened` and its local state
- [x] 2.5 Link issue and request numbers: terminal hyperlinks on a terminal, each only in its own cell; `-urls` for an aligned plain list, which needs `-table`
- [x] 2.6 An issue is identified by repository and number: a closed issue in another repository is shown with its repository and its own page
- [x] 2.7 Fetch reviewers in one open-only call, only for `-table` and `-json`; cache entries record whether they hold them; unknown reviewers read `Unknown`
- [x] 2.8 Print related-work warnings under the table; reject `-table` with `-json`
- [x] 2.9 Sanitize cell text, link only safe URLs, name the owner when short repository names clash, treat repository names case-insensitively
- [x] 2.10 Count display columns, and fit the real terminal width

## 3. Tests

- [x] 3.1 One golden table covering approved, review requested, changes requested, failing checks, pending checks, no checks, no pull request and merged
- [x] 3.2 Several issues on one pull request; the repository is named when rows span repositories or the issue is in another one
- [x] 3.3 The new fields round-trip through JSON; an old cache entry without them still loads; reviewers parse from `gh` JSON (user and team requests, commented and dismissed reviews)
- [x] 3.4 Hyperlinks add no width and appear only when asked; an issue and a request with the same number link to their own pages; a number inside a longer one is not linked
- [x] 3.5 A request with no state renders as Unknown; `-urls` without `-table` and `-table` with `-json` fail
- [x] 3.6 Reviewer standing from `gh` JSON, the open-only call and its failure, local states, control characters, unsafe URLs, repository case, clashing short names, related warnings, wide characters

## 4. Docs

- [x] 4.1 Add the flags as part of the existing commands per `make-commands-discoverable`; README, `agent-help` and the skill are generated
- [x] 4.2 Update `site/features.json`
- [x] 4.3 Create the issue, reference it in the commits, run `specsync changelog -fail-on-unlinked-commits`
