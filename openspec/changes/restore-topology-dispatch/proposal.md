# Restore the topology command and fix stale "soon" badges on the site

## Why
PR #174 (store support) was rebased from a branch that predated two pieces of
already-shipped work: the `topology` subcommand's dispatch wiring in
`cmd/specsync/main.go`, and the "See who's already working what" feature card
on specsync.se. The flat-diff reapplication silently dropped the `topology`
case from `knownSubcommands` and the `main()` switch, so `specsync topology`
now fails with "unknown subcommand" even though `runTopology` and all its
tests are still present and working.

Separately, while reviewing the site after that merge: three feature cards
(design notes overflow #139, topology #168, adopt #160) still carry a "soon"
badge even though all three shipped weeks ago.

## What Changes
- Restore `"topology": true` in `knownSubcommands`, the `case "topology":
  runTopology(rest)` dispatch arm, and `topology` in the subcommand list
  printed on an unknown-subcommand error.
- Drop the stale `"status": "soon"` / `"issue"` fields from the three
  already-shipped feature cards in `site/features.json`.

## Impact
- Affected code: `cmd/specsync/main.go`, `site/features.json`
- No spec delta: restores previously-specified, previously-tested behavior
  that regressed in a merge; doesn't change any contract.
