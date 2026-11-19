# Show work as a status table with reviews and checks

## Why

Asking "where does this work stand?" means one question per issue and per pull request: is it open, do the checks pass, who must review it, is it approved. `status` answers part of that as prose for one branch, and `overview` lists rows without review detail. Neither prints the one table a person pastes into a chat or a stand-up, and neither says who has been asked to review.

## What

- `specsync status -table` prints one row per issue and pull request tied to the current work: the change's issue, the issues related to it, and every pull request that closes one. Columns: issue, what, pull request, state, checks, review, size.
- Each pull request carries the reviewers it requested and each reviewer's latest verdict, so the review column reads `approved`, `changes requested`, or `review requested from <login>`.
- Issue and pull request numbers link to their pages: as terminal hyperlinks when output is a terminal, and with `-urls` as a plain `label  url` list under the table, for pasting into chat or a pull request. An issue in another repository than its request's is named with that repository.
- Reviewers come from one GraphQL call per repository among the rows shown (in batches), naming exactly the pull requests shown, and only for `-table` and `-json`, so `status`, `overview` and `cleanup` cost what they did and related work in other repositories is not fetched. Requested reviewers include bots such as Copilot and teams. A reviewer's standing is their latest approval, change request or dismissal (`latestOpinionatedReviews`), so a later comment does not hide an approval and a busy request does not run into the 100-review cap of `gh pr list`. The result is cached for a minute apart from the listing, under the listing's `-state` rules (on reuses, fresh refetches, off asks nobody). A request the provider did not return, or a list it cut short, is said under the table; reviewers that were not fetched read `Unknown`. A row whose pull request lookup did not happen or failed says `Unknown` in the PR column, row by row, not `Not opened`; a change with no branch says `No branch`. A closed issue says so in the Issue column (`#20 closed`).
- Under the table: how many rows have no pull request lookup, the reasons a source was incomplete that concern the repositories shown, and related items that resolved to no row, each said once.
- Text from the tracker is cleaned of control characters, only http(s) URLs become links, bidirectional and other format characters are dropped, column widths count terminal columns (wide characters and emoji take two), repositories whose short names clash anywhere in the table are written with their owner, and the width follows `$COLUMNS` or the terminal.
- `-json` carries the same fields, so an agent can build any other view.
- `overview -table` prints the same table for every repository it covers.
- A row with no pull request says so (`not opened`) and still shows the local state of the branch.
- Checks read `all N pass`, `N pass, M failing (names)`, `N pass, M pending`, or `none` when the pull request has no checks. Size reads `N files, +A -D`.

## Out of scope

- `-table` and `-json` cannot be combined.
- It never changes an issue, a branch or a pull request.
- It does not judge whether work is done; it shows the states the providers report.
- Trackers without pull requests keep working: the review and checks columns stay empty.

## Release note

`specsync status -table` and `specsync overview -table` print the work, its issues and pull requests as one table with checks, requested reviewers and review state.
