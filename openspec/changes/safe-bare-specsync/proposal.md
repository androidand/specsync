# Running specsync with no command never writes

## Why

`specsync` with no arguments syncs every change in the repository, and so does
any invocation that starts with a flag and names no change. An agent exploring
the CLI ran bare `specsync` to see its commands and created 26 issues in a
repository it was only meant to document, one per open or archived change, and
rewrote four existing issue bodies. Undoing it took another round of tracker
writes. A command that writes to a shared tracker must not be the default for
an empty or exploratory invocation.

## What Changes

- `specsync` with no arguments, and `specsync -h`, `-help`, `--help` or `help`,
  print the commands grouped by what they are for, then exit 0. Nothing is
  read from or written to the tracker.
- A sync that would write and names no change is refused, with a message
  naming `-change <slug>`, `-all` and `-dry-run`. This is the rule stores
  already follow (`guardStoreScope`), now applied to every root.
- `-dry-run` without a change still previews every change: it writes nothing.
- The unknown-subcommand error no longer says that running with no subcommand
  syncs.

## Capabilities

### New Capabilities

- `cli-safety`: the empty invocation and unscoped syncs never write to the
  tracker.

## Impact

- `cmd/specsync/main.go` (dispatch, usage), `cmd/specsync/cmd_sync.go`,
  `cmd/specsync/root.go`.
- Breaking for scripts that relied on bare `specsync` or a flag-only call
  syncing everything: they add `-all`.
- README, skill, agent-help.

## Release note

`specsync` with no command prints its commands instead of syncing; syncing
every change now needs `-all`.
