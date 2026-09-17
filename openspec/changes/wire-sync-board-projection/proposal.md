# Wire sync's -project flag to actually project onto the board

## Why

`sync -project owner/N` (or `openspec/specsync.yml`) resolves a
`BoardTarget`, threads it through `Options.Project`, and `Sync` reports
`ItemResult.BoardConfigured` from it — but never calls `ProjectOntoBoard`.
Only `pull` does (`pull.go:134`). The result: `sync -project` runs clean,
reports success, `BoardConfigured: true`, and silently does nothing to the
board. `printBoardPlan` (`cmd/specsync/main.go:887`) prints nothing useful
either, since it's handed a permanently-zero-value `BoardPlan` — so there's
no error, no warning, just quiet non-function exactly matching the
documented, expected behavior in shape (`BoardConfigured` says true) while
doing none of it.

**This is a regression, not a gap.** `git log -S ProjectOntoBoard -- sync.go`
shows `Sync` used to call it, inline in a per-provider `syncOne` helper, with
a hand-rolled three-way-merge dance (query current board state, compare
against a saved binding, decide push-local vs. respect-human-move vs.
report-conflict) before deciding whether to call `ProjectOntoBoard` at all.
Commit `4177dc7` ("multi-provider sync (fan-out)") restructured `Sync` into
its current per-provider loop and dropped that whole block — `Board:` and
the `ProjectOntoBoard` call vanished from the diff; `BoardConfigured:
opts.Project.Configured()` and the `Board BoardPlan` field on `ItemResult`
were left behind, reporting stale/empty data ever since. No test caught it:
`board_test.go` only calls `ProjectOntoBoard` directly, never through
`Sync`; `TestBoardTargetCarriesStatusMapping` (added for an earlier,
narrower version of this same class of bug — `BoardTarget.StatusMapping`
reaching the CLI) only checks target construction, not that projection runs.

**The fix is smaller than the regression, though.** `ProjectOntoBoard`
itself (`board.go:98`) has been rewritten since the fan-out refactor into a
fully self-contained operation: it resolves the board schema, detects human
moves in both directions (`HumanMovedToDone`/`HumanMovedToActive`),
protects against clobbering a human-set Status via
`LastWrittenOptionID`-based reasoning, and saves its own board binding
(`changeDir` param) — internally, on every call. None of the external
three-way-merge machinery the old `syncOne` had to hand-roll is needed
anymore; `pull.go` already proves this by calling `ProjectOntoBoard` with a
single line. Restoring the call to `Sync` is that same single line, not a
rebuild of the old logic.

## Proposed Changes

- In `Sync`'s per-provider loop (`sync.go`), after `Push` succeeds, call
  `bp.ProjectOntoBoard(ctx, opts.Project, ref, item, opts.DryRun, c.Dir)`
  when `opts.Project.Configured()` and the provider implements
  `BoardProjector` — mirroring `pull.go`'s existing call exactly. It already
  handles dry-run internally (a zero-API preview), so no extra dry-run
  branching is needed here.
- Capture the first successful plan into `ItemResult.Board` (first-provider-
  wins, matching the existing `firstRef`/`firstCreated` pattern for
  URL/created reporting) so `printBoardPlan` finally has real data.
- A `ProjectOntoBoard` error is non-fatal to the change's sync (the issue
  itself already pushed successfully) but printed as a warning to stderr —
  silence is exactly the failure mode this proposal exists to remove, so a
  new failure path here must not repeat it.

## Non-Goals

- **Not** touching `DepSync`'s existing error handling
  (`sync.go`, dependency-edge reconcile) even though it has the same
  silent-swallow shape (`if _, err := DepSync(...); err != nil { /* Log and
  continue */ }` — no logging actually happens). Same bug family, separate
  fix, separate PR.
- **Not** reintroducing the old inline three-way-merge dance from before
  the fan-out refactor — superseded by `ProjectOntoBoard`'s own internal
  handling, per the Why section.
- **Not** per-provider board reporting in `ProviderResult` for multi-board
  fan-out (multiple providers simultaneously implementing `BoardProjector`)
  — first-provider-wins is enough today since GitHub is the only
  `BoardProjector` implementation that exists.

## Open Questions

1. Should a `ProjectOntoBoard` failure also skip `DepSync` and any later
   post-push step for that provider in the same run, or are they
   independent enough to both attempt regardless? Leaning independent —
   they touch unrelated state (board vs. issue cross-links).

## Release Notes

Fixed: `sync -project` (and `openspec/specsync.yml`'s board target) now
actually projects issues onto the configured GitHub Projects board — it
previously reported `BoardConfigured: true` and silently did nothing.
`pull` was unaffected; this restores parity for `sync`.
