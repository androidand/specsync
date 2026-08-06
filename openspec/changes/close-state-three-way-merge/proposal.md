# Closing an issue is a review decision, not a spec-push side effect

## Why

`-close-completed` reads as one-way — "close the item once every task is checked".
It isn't. `sync.go` sets `ManageClosed: c.Archived || closeCompleted`, and the
GitHub provider acts on it in both directions:

```go
if item.ManageClosed && !item.Closed && currentlyClosed {
    return *existing, p.reopen(ctx, num)
}
```

So with the flag on, specsync claims authority over open/closed state. The failure
that follows is concrete: work ships, a merged PR (or a human, or a reviewing
agent) closes the issue, and then the next push touching `openspec/**` **reopens
it** — because the change's stage isn't `complete` and specsync asserts its own
view. The close is undone by an unrelated spec edit.

Closing an issue is a judgement about whether work is done *and reviewed*. That
judgement belongs to whoever merges the PR. It does not belong to a path filter on
a spec push, and specsync should not overrule it.

specsync already knows this everywhere else. Board status runs a three-way merge
against a recorded base and refuses to clobber — `"human moved the card on the
board; specsync won't clobber it"`. Task state merges as a monotonic union so a
lagging issue can never uncheck local progress. The provider doc comment states
the principle outright: *"it is glue, not a control plane, and not a second
authority."* Issue open/closed state was the one place that principle wasn't
enforced.

The existing "Reversible completion state" requirement is not wrong in intent —
reopening when new work appears after completion is genuinely useful. It is wrong
in that it cannot tell *new work appeared* from *someone else closed this*.

## What Changes

- `Ref` gains `BaseClosed *bool`: the open/closed state specsync itself last
  asserted — the merge base for open/closed, exactly as `Base` is for `tasks.md`.
  It lives in the gitignored ref cache and is disposable. `nil` (never asserted,
  or cache discarded) is deliberately distinct from `false`.
- The reopen is gated on that base. Only a base of `true` licenses it: "closed" was
  specsync's own last word and local work has since reappeared. A base of `false`
  or `nil` means the close came from outside, and specsync leaves it alone.
- An external close is **not** adopted as the new base, so specsync stays
  deferential rather than re-arming itself to reopen on a later run. Whoever took
  over the state keeps it until specsync closes the item again itself.
- Closing is unchanged: still one call, and it now records the base.
- **BREAKING** for anyone relying on unconditional reopen-on-new-work: an issue
  closed outside specsync is never reopened automatically.
- `-close-completed` comes out of `.github/workflows/sync.yml`. CI keeps issue
  *content* current; open/closed state is the tracker's own.

## Non-Goals

- **Surfacing the skip in run output.** When specsync defers to an external close
  it currently says nothing. Doing this properly means the plan-shaped reporting
  the board path has (`BoardPlan.StatusSkipped`), which needs the decision and its
  reason to reach the core — a `Push` contract change. Bolting on a half-version
  would be worse than the silence. Deferred deliberately, not overlooked.
- **Reading an external close back into local stage.** The clean end-state is that
  a PR-driven close reconciles inbound into `.status`/metadata, the way checkbox
  state already flows back. That is a new inbound path with its own design
  questions and is out of scope here.

## Impact

- Affected code: `provider.go` (`Ref.BaseClosed`), `github.go` (`Push` gate),
  `sync.go` (base preservation), `.github/workflows/sync.yml`.
- Affected specs: `completion-lifecycle` — replaces "Reversible completion state".
- Migration: none required. An existing cache has no `base_closed`, so the first
  sync after upgrading treats every issue as "not specsync's close" and defers.
  specsync re-arms the moment it closes an item itself.
- Note: `completion-lifecycle` was never promoted from
  `changes/archive/fix-completion-lifecycle/` into `openspec/specs/`, so this
  delta targets a capability the accepted spec set does not yet carry. Worth
  fixing, but not in this change.
