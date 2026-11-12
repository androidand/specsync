# Run the gates before a push when they are stale

## Why

Gates pass on a commit and leave a stamp. A push can still send a commit nobody checked. Running the gates by hand is easy to forget.

Running every gate on every push is slow when the stamp already names the commit.

## What

- `specsync gates -if-stale`: do nothing and exit 0 when the stamp names HEAD on a clean tree. Otherwise run the gates as usual.
- `-if-stale` with `-only` is an error, because a partial run never stamps.
- A short recipe in the docs for a `pre-push` hook that runs `specsync gates -if-stale`.

## Out of scope

- Installing hooks. A hook is a repo or machine choice.
- Blocking `git push --no-verify`. That stays git's own escape.

## Release note

Run `specsync gates -if-stale` in a pre-push hook to check a commit only when it has no passing stamp.
