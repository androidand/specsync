# Tasks

## Implementation

- [ ] `LoadChangeBySlug` (change.go): fall back to a date-prefix-aware glob of
  `changes/archive/*` (match `<slug>` or `<YYYY-MM-DD>-<slug>`) instead of an
  exact-path check
  - [ ] Multiple matches is an error listing the candidate folders
- [ ] `LoadChange` (change.go): strip a leading `YYYY-MM-DD-` prefix from
  `filepath.Base(dir)` when computing `Slug` for archived changes, so `Slug`
  is stable across the active → archived transition
- [ ] `resolveEntry` (change.go, links.md resolution): apply the same glob
  resolution to the `changes/archive/<slug>` candidate path

## Testing

- [ ] Unit test: `LoadChangeBySlug` finds a change at
  `changes/archive/2026-08-10-<slug>/` when queried by `<slug>`
- [ ] Unit test: archived `Change.Slug` equals the original slug, not the
  date-prefixed folder name
- [ ] Unit test: `LoadChangeBySlug` errors clearly on ambiguous matches
  (two archived folders both ending in `-<slug>`)
- [ ] Unit test: a `links.md` slug entry still resolves after its target is
  archived
- [ ] Regression test: `specsync -change <slug>` after `openspec archive
  <slug> -y` succeeds and projects `StageArchived` to a configured board

## Validation

- [ ] `make test` passes
- [ ] `make vet` passes
- [ ] Manual repro: archive a change in this repo, confirm
  `specsync -change <slug>` (same original slug) no longer errors
