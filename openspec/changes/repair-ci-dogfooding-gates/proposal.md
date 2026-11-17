# Repair CI dogfooding gates

## Why

AGENTS.md promises that CI fails a PR with an unlinked commit, because such a commit degrades
`CHANGELOG.md` to a raw `<description> (<hash>)` line. The workflow ran `changelog -resolve-refs`
without `-fail-on-unlinked-commits`, so the promise was not enforced. It also checked out a
shallow clone with no tags, so the default range (latest tag to HEAD) could not be computed.

An earlier indentation error that made a step unparseable (#119) is already fixed; nothing
prevented it from coming back.

## What

- CI checks out full history and tags, and runs `changelog -resolve-refs -fail-on-unlinked-commits`.
- A test reads `.github/workflows/ci.yml` and fails if the gate flag is missing, full history is
  not fetched, or a step's indentation differs from its siblings.

## Non-Goals

- Changing what counts as an unlinked commit.

## Release note

CI now fails a pull request that contains a commit not linked to its change's issue.
