# Close an archived change as not planned or as a duplicate

## Why

`archive` closes the issue with `gh issue close` and no reason, so every archived change closes as
completed. A change that was abandoned, or that duplicates another, has no honest way through: `archive`
refuses unchecked tasks unless `-force`, and even then it records the work as completed. Closing those by
hand outside specsync leaves the change in place (see `changes-show-issue-state`).

## What

- `archive -reason completed|not-planned|duplicate` (default `completed`, as today).
- `archive -duplicate-of <ref>` is required with `-reason duplicate`, and rejected otherwise. The ref is an
  issue number or URL.
- `-reason not-planned` and `-reason duplicate` do not require checked tasks, so they need no `-force`.
- `-comment <text>` is optional and is posted as the closing comment.
- The provider `close` takes a reason. GitHub maps the reason to `gh issue close --reason` and
  `--duplicate-of`; a provider with no such reason closes as it does today and prints a note.
- The closed-state base recorded for the three-way merge is the same as today, so the later sync still
  defers to the close.

## Out of scope

- No bulk archive.
- Reopening is unchanged.

## Release note

`specsync archive -reason not-planned|duplicate` closes an abandoned or duplicate change with the matching reason, without `-force`.
