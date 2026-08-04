# links.md is append-only

## Why

`links.md` is documented as "the human- and agent-readable source of relationship
truth", and people treat it that way: they write dependency order, sequencing
notes, and prose about *why* two changes relate, alongside the `## Blocked by` /
`## Blocks` sections the dependency sync maintains.

But every write path — `link`, `pull`, `spinoff` — went through a
`saveLinksToMD` that rewrote the whole file as a bare list of `- owner/repo#N`
lines. Recording one new link destroyed everything else in the file.

Reported from actual usage: a maintainer declined to run `specsync link` at all,
because doing so would have wiped the dependency order and sequencing notes they
had written by hand. A tool that authors must avoid to protect their own notes has
inverted its job. The same trap sits in `pull` — a routine re-pull silently
flattens the file — and in `spinoff`.

The file is not specsync's to own. specsync adds the correspondence it discovers;
what else lives there, and what gets removed, is the author's call.

## What Changes

- `saveLinksToMD` appends only the refs not already recorded, preserving the rest
  of the file verbatim.
- Deduplication compares *resolved* refs, so a full URL, its `owner/repo#N`
  shorthand, and a sibling slug that resolves to it all count as the same link —
  no near-duplicate entries, and no double render in the issue's `## Related`.
- A ref set that is already fully recorded writes nothing at all, so repeat runs
  leave no diff to review.
- From-scratch output is unchanged: a change with no `links.md` still gets a plain
  list, with no section header imposed on it.

## Impact

- Affected code: `cache.go` (`saveLinksToMD`), call sites in `link.go`, `pull.go`,
  `spinoff.go` (each now passes the openspec dir so slug entries resolve).
- No format change and no migration: existing bare-list `links.md` files keep
  their shape.
- Behavior note: because removal is no longer specsync's call, a link deleted
  locally and still present on the issue is re-appended by the next `pull`.
  Removing a link is a local edit plus a push, matching how the file's other
  content already works.
