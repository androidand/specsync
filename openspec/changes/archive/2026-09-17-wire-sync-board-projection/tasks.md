# Tasks

- [x] 1. In `Sync`'s per-provider loop (`sync.go`), after `ref, perr :=
      prov.Push(...)` succeeds, add: when `opts.Project.Configured()` and
      `prov` implements `BoardProjector`, call
      `bp.ProjectOntoBoard(ctx, opts.Project, ref, item, opts.DryRun, c.Dir)`.
      Track the first successful plan into a `boardPlan BoardPlan` local
      (mirroring the existing `firstRef`/`firstCreated` pattern), and set
      `ItemResult.Board: boardPlan` in the `res.Items = append(...)` call
      that already sets `BoardConfigured`.
      Validation: a fixture `Sync` call with a configured `BoardTarget`
      against a `BoardProjector`-implementing fake provider results in a
      `ProjectOntoBoard` call being made (mirrors `pull_test.go`'s existing
      coverage of the same call via `Pull`) and `ItemResult.Board` non-zero.

- [x] 2. A `ProjectOntoBoard` error is non-fatal: print
      `fmt.Fprintf(os.Stderr, "specsync: warning: board projection for %s
      failed: %v\n", c.Slug, err)` and continue the rest of that provider's
      post-push steps (dependency sync, etc.) rather than `continue`-ing
      past them. Add the `os` import to `sync.go`.
      Validation: a fixture where the fake `BoardProjector` returns an error
      still completes the sync (issue created/updated, no early exit) and
      the warning is observable (captured stderr, or the function signature
      exposes it for the test — match whatever pattern existing warnings in
      this codebase use, e.g. `pull.go`'s intake-transition warning).

- [x] 3. Regression test proving the specific regression this proposal
      describes: `Sync` with `Options.Project` configured against a
      `BoardProjector` fake provider must result in exactly one
      `ProjectOntoBoard` call per change per provider — not zero (the bug),
      not more than one.

- [x] 4. Dry-run coverage: `Sync` with `DryRun: true` and a configured board
      target still calls `ProjectOntoBoard` (it handles dry-run internally,
      zero API calls) and the resulting `ItemResult.Board` is non-zero, so
      `-dry-run` output actually previews the board plan instead of the CLI
      printing nothing under "board:".

- [x] 5. Manual/live verification: ran `specsync -dry-run -change
      wire-sync-board-projection -project androidand/2` against a real
      GitHub Projects board with both the unpatched and patched binary.
      Unpatched: only the static "would ensure the issue is on the board"
      line printed (StatusName/AssigneeLogin always empty). Patched: all
      three preview lines printed (add/status/assignee), confirming the
      exact silent-no-op this proposal describes and its fix. Did not
      perform a real (non-dry-run) write against that board — it is an
      unrelated project (BrickNow Prioritization), not a low-stakes test
      board for this repo, so a live mutation was skipped pending a real
      target.

- [x] 6. No documentation change needed for accuracy — `README.md:592-599`
      ("sync + project onto board 6") and `site/features.json`'s "Projects
      board sync" entry already correctly describe the intended behavior;
      this proposal makes reality match what's already documented, not the
      other way round.
