# Keep appended links out of dependency sections

## Why

Since `append-only-links-md`, `saveLinksToMD` appends new entries at the end of
`links.md`. `parseLinksMD` assigns each entry to the section heading above it.
So when the file's last section is `## Blocked by` (or `## Blocks`), an entry
appended by `specsync link`, `pull`, or `spinoff` is read back as a dependency,
not a related link. Because dependency sections are synced to the tracker, the
next sync can record that issue as blocking.

Reproduced on `main` (v0.14.1) with a test that writes
`## Blocked by\n- owner/repo#2\n`, calls `saveLinksToMD` with `owner/repo#5`, and
parses the file back: `blockedBy` holds #2 and #5, `related` is empty.

The same happens under any heading specsync does not own: entries land under the
author's last heading. Authors and tools that keep their own sections need
related links to stay out of them.

## What Changes

- When the existing file's last section heading is anything other than
  `## Related`, `saveLinksToMD` writes a `## Related` heading before the
  appended entries. Existing bytes are still never changed, so the file stays
  append-only.
- A file with no headings, or whose last section is `## Related`, is appended to
  as today. From-scratch output stays a plain list.

## Non-goals

- No reordering or moving of existing entries; an entry already misplaced by the
  bug is the author's to move.
- No change to how `## Blocked by` / `## Blocks` are written by dependency sync.

## Discoveries

- `TestPull_CannotResolveFromBranch` reads the real git branch of the checkout it
  runs in. On any branch named `feat/<n>-...` (this repo's own convention) it
  resolves an issue and fails; it passes on `main` and in CI's detached checkout.
  Reproduced on clean `main` with a throwaway `feat/1-...` branch. Unrelated to
  this change; worth its own fix so the suite is green on feature branches.

