# Tasks

## Implementation

- [x] `LoadChangeBySlug` (change.go): fall back to a date-prefix-aware glob of
  `changes/archive/*` (match `<slug>` or `<YYYY-MM-DD>-<slug>`) instead of an
  exact-path check
  - [x] Multiple matches is an error listing the candidate folders
- [x] `LoadChange` (change.go): strip a leading `YYYY-MM-DD-` prefix from
  `filepath.Base(dir)` when computing `Slug` for archived changes, so `Slug`
  is stable across the active → archived transition
- [x] `resolveEntry` (change.go, links.md resolution): apply the same glob
  resolution to the `changes/archive/<slug>` candidate path

## Testing

- [x] Unit test: `LoadChangeBySlug` finds a change at
  `changes/archive/2026-08-10-<slug>/` when queried by `<slug>`
- [x] Unit test: archived `Change.Slug` equals the original slug, not the
  date-prefixed folder name
- [x] Unit test: `LoadChangeBySlug` errors clearly on ambiguous matches
  (two archived folders both ending in `-<slug>`)
- [x] Unit test: a `links.md` slug entry still resolves after its target is
  archived
- [x] Regression test: `specsync -change <slug>` after `openspec archive
  <slug> -y` succeeds and projects `StageArchived` to a configured board
- [x] Regression test: `Find`/`ResolveLiveRefs` (github.go, gather.go) locate
  an archived change's issue by its pre-archive marker with no local ref
  cache present — pins the fix for the empty-changelog/duplicate-issue
  incident on `sync-design-notes` (#139/#141)

## Validation

- [x] `make test` passes
- [x] `make vet` passes
- [x] Manual repro: archive a change in this repo, confirm
  `specsync -change <slug>` (same original slug) no longer errors
