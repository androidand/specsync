# Fix archived-slug mismatch breaking live ref resolution

## Why

Reproduced 2026-08-27/28 shipping `sync-design-notes` (#139): `openspec archive
<slug> -y` renames the change folder to a date-prefixed slug
(`sync-design-notes` → `2026-08-27-sync-design-notes`), but the identity
marker already embedded in the tracker issue's body
(`<!-- specsync:change=sync-design-notes -->`) keeps the pre-archive slug —
it's only rewritten if something re-syncs that exact change again after
archiving, which normally never happens (the change is done; nothing pushes
to it again).

That drift is invisible as long as a local `.specsync/refs.json` cache
exists, because `GatherTrace`/`Sync` read the cached issue id straight from
disk and never need to search by slug. It breaks the moment there's no
cache — which is every fresh CI checkout, since `.specsync/` is gitignored
by design:

- `cmd/specsync changelog -resolve-refs` calls `ResolveLiveRefs` →
  `WorkProvider.Find(ctx, slug)`, which searches for
  `specsync:change=<slug>` using the **current on-disk slug** (now
  date-prefixed). It doesn't match the issue's still-pre-archive marker, so
  the change is silently treated as unresolvable, its commits fall through
  to "loose", and — because they're not conventional-commit-prefixed
  fallback material either — they render as nothing at all. This produced
  an empty `v0.13.0` GitHub Release body and an empty `CHANGELOG.md`
  section for a release that very much shipped something.
- `.github/workflows/sync.yml` runs a full `specsync` (no cache, no
  `-change`) on every push to `main` touching `openspec/**`. For the same
  reason, it failed to find #139 for the newly-archived change and created
  a duplicate issue (#141) instead — on a change that was already merged,
  already had its own issue, and had nothing left to sync.

Both failures trace to the same root cause: **live resolution searches by
the change's current slug, but the marker was written under a slug that
archiving later changed.**

## Proposed Changes

- Make `Find`'s slug search account for the archived date-prefix rename
  (e.g. strip a leading `YYYY-MM-DD-` when the change is archived and
  retry, or search both forms) — or, better, re-embed the marker under the
  new slug at archive time itself, so drift never exists in the first
  place. The two approaches aren't mutually exclusive; the second closes the
  gap at the source, the first is a resilient fallback for changes already
  archived before this fix ships.
- `sync.yml` (or `Sync` itself) should refuse to *create* a new issue for a
  change that is `Archived` and already closed/complete without a much
  higher-confidence "this really doesn't exist yet" signal — an archived
  change failing to resolve its issue is far more likely to be exactly this
  drift than a genuinely new item.
- Add a regression test: an archived change whose folder was renamed after
  its issue's marker was written still resolves via `Find`/`ResolveLiveRefs`
  with no local ref cache present.

## Capabilities

### Modified Capabilities

- `stable-projection-identity`: rediscovery by marker must survive an
  archive rename (date-prefixed slug) even with no local ref cache, not
  just cache loss on an unchanged slug.

## Non-Goals

- Not re-architecting how `openspec archive` names folders — that's
  upstream OpenSpec CLI behavior, out of scope here.
- Not a general audit of every other place slugs are compared — scope is
  the live-resolution path (`Find`, `ResolveLiveRefs`, and whatever
  `sync.yml`'s full-repo run relies on) that this incident actually hit.

## Impact

- `github.go` (`Find`), `gather.go` (`ResolveLiveRefs`), possibly
  `sync.go`/`archive.go` for re-marking at archive time.
- `changelog -resolve-refs` and `sync.yml`'s fresh-checkout behavior for
  every future archived change, not just this one.
