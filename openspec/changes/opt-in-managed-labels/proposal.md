# Make specsync/stage labels opt-in, off by default

## Why

Every sync currently adds `specsync` and `stage:<stage>` labels to every
issue (`desiredLabels`, `github.go`), unconditionally. Neither is consumed
anywhere in specsync itself:

- Issue identity is resolved entirely via the `<!-- specsync:change=<slug>
  -->` body marker, never the `specsync` label.
- `stage:<stage>` duplicates the Projects board Status column wherever a
  board is actually projected (`ProjectOntoBoard`, `board.go`).

So today they're pure GitHub-UI affordances — useful in a repo where issues
come from many sources and you want to filter to "just the specsync ones,"
of no discriminating value at all in a repo (like this one) where every
issue already is a specsync issue. Reported as noise.

**Aside, found while investigating:** `sync`'s `-project` flag currently
only affects the printed `BoardConfigured` report — `Sync` (`sync.go`) never
calls `ProjectOntoBoard`; only `pull` does. So "skip the stage label when a
board is configured" isn't a safe default today — for anyone using `sync
-project`, no board write happens either, and skipping the label would
leave stage invisible everywhere. That gap is real but out of scope here;
this proposal doesn't fix it, and doesn't lean on it being fixed.

## Proposed Changes

- Add `WorkItem.ManagedLabels bool` (default `false`, i.e. zero value).
  `desiredLabels` adds `specsync` and `stage:<stage>` only when true;
  `priority:<n>` is unaffected (added whenever `Priority > 0`, opt-in or
  not — it's a distinct feature, not the reported noise).
- `Sync` sets `item.ManagedLabels = opts.Labels` after `WorkItemFor`.
  `Options.Labels bool` is new, default `false`.
- CLI: new `-labels` flag on `sync` (default `false`). Passing it restores
  the previous always-on behavior for orgs that do want native-label
  triage in a mixed-source repo.
- **Self-cleaning, not just stop-adding**: `managedLabel()` still recognizes
  `specsync`/`stage:`/`priority:` as reconcilable. An issue that already
  carries `specsync`/`stage:x` from before this change, synced again
  without `-labels`, has them *removed* on that sync (no longer in
  "desired", still in the managed namespace) — not just held steady. This
  is the intended behavior, not a side effect to guard against.

## Non-Goals

- **Not** touching `epic.go`'s explicit `Labels: []string{"specsync",
  "type:epic"}` override on coordination issues — a distinct, deliberate
  choice for a different feature, unaffected either way since it bypasses
  `desiredLabels`'s default construction entirely.
- **Not** touching the `idea`/`ideas` feature's `stage:intake` label
  (`runIdea`/`runIdeas`, `cmd/specsync/main.go`) or `pull`'s `stage:intake`
  → `stage:active` transition (`pull.go`). That's a separate, load-bearing
  mechanism — `specsync ideas` finds its issues *by* that label — not the
  decorative per-sync labeling this proposal is about. It never went
  through `desiredLabels` and stays unconditional either way.
- **Not** fixing the `sync -project` board-projection gap noted above.
- **Not** changing `pull`'s behavior otherwise — pull never wrote
  `specsync`/`stage:<stage>` via `desiredLabels` in the first place.

## Release Notes

`specsync` and `stage:<stage>` labels are no longer added by default —
issue identity was never based on them (the body marker is), and stage
duplicates the Projects board Status column wherever a board is used. Pass
`-labels` to `sync` to restore the previous always-on behavior. A sync
without the flag also removes these labels from issues that already carry
them from before this change.
