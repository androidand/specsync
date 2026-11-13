# Show work as a status table with reviews and checks

## Why

Asking "where does this work stand?" means one question per issue and per pull request: is it open, do the checks pass, who must review it, is it approved. `status` answers part of that as prose for one branch, and `overview` lists rows without review detail. Neither prints the one table a person pastes into a chat or a stand-up, and neither says who has been asked to review.

## What

- `specsync status -table` prints one row per issue and pull request tied to the current work: the change's issue, the issues related to it, and every pull request that closes one. Columns: issue, what, pull request, state, checks, review, size.
- Each pull request carries the reviewers it requested and each reviewer's latest verdict, so the review column reads `approved`, `changes requested`, or `review requested from <login>`.
- `-json` carries the same fields, so an agent can build any other view.
- `overview -table` prints the same table for every repository it covers.
- A row with no pull request says so (`not opened`) and still shows the local state of the branch.
- Checks read `all N pass`, `N pass, M failing (names)`, `N pass, M pending`, or `none` when the pull request has no checks. Size reads `N files, +A -D`.

## Out of scope

- It never changes an issue, a branch or a pull request.
- It does not judge whether work is done; it shows the states the providers report.
- Trackers without pull requests keep working: the review and checks columns stay empty.

## Release note

`specsync status -table` and `specsync overview -table` print the work, its issues and pull requests as one table with checks, requested reviewers and review state.
