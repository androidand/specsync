# Resolve archived changes by their original slug

## Context

`openspec archive <slug> -y` (the `openspec` CLI, not specsync) moves
`openspec/changes/<slug>/` to `openspec/changes/archive/<YYYY-MM-DD>-<slug>/`
— it prepends a date to the folder name. specsync never accounts for that
prefix, which breaks change resolution by slug in two related ways once a
change is archived:

1. **`specsync -change <slug>` fails outright after archiving.**
   `LoadChangeBySlug` (change.go) falls back to archived changes by checking
   exactly `openspec/changes/archive/<slug>/`:

   ```go
   dir = filepath.Join(openspecDir, "changes", "archive", slug)
   c, err = LoadChange(dir, true, openspecDir)
   ```

   That path never exists — the real folder is
   `changes/archive/2026-08-10-<slug>/`. The lookup returns `(nil, nil)` from
   `LoadChange` (no `proposal.md` at that path), so `LoadChangeBySlug` reports
   `no change found for slug %q`. Any `specsync -change <slug>` run after
   archiving — which is exactly what this repo's own `AGENTS.md` "Completion
   Rule" and the published skill's "Complete a change" workflow describe as
   the last step, sync-then-archive — has no working follow-up. There is no
   way to re-sync an archived change by its slug at all today.

2. **The change's own `Slug` field silently changes on archive**, independent
   of bug 1. `LoadChange` derives the slug from the folder name:

   ```go
   slug := filepath.Base(dir)
   ```

   For an archived change this evaluates to the *date-prefixed* folder name
   (`2026-08-10-<slug>`), not the original slug the change was created,
   worked, and referenced under. `LoadChanges` (used by `specsync
   changes --json` and the audit/changelog paths) enumerates archived changes
   under this new identity — any script, memory, or documentation that
   tracked the change by its original slug can no longer find it by name in
   bulk listings either, without knowing to strip the date.

3. **`resolveEntry` (links.md resolution) has the identical exact-path bug**
   for slug-based cross-links:

   ```go
   for _, dir := range []string{
       filepath.Join(openspecDir, "changes", slug),
       filepath.Join(openspecDir, "changes", "archive", slug),
   } {
   ```

   A `links.md` entry naming a sibling change by slug silently stops
   resolving the moment that sibling is archived — the link just goes quiet
   (per the documented "unresolvable slug entries are silently skipped"
   behavior), with no signal that the cause is the date prefix rather than a
   genuinely missing sibling.

### How this was found

Discovered while closing out `portal-resource-select-ui` (portal#4212 /
FusionHub#3565, both merged): after archiving the change locally with
`openspec archive portal-resource-select-ui -y`, the tracked GitHub issues
(portal#4210, FusionHub#3538) never reflected the terminal `archived` stage —
board Status stayed at "Ready for development" indefinitely on the Backlog
project, well after the underlying work had shipped. The board fix was done
by hand via direct API calls rather than through specsync, since the
documented sync-after-archive path doesn't work.

## Proposed Changes

Make archive-folder resolution date-prefix-aware everywhere a slug is
resolved against `changes/archive/`, instead of assuming an exact match:

1. **`LoadChangeBySlug`**: when the exact `changes/archive/<slug>` path
   doesn't exist, glob `changes/archive/*` for a directory whose name equals
   `<slug>` or matches `<YYYY-MM-DD>-<slug>` (the openspec CLI's own naming
   pattern), and load whichever one is found. Ambiguous (multiple matches) is
   an error naming the candidates, not a silent pick.
2. **`LoadChange`**: when computing `Change.Slug` for a folder under
   `changes/archive/`, strip a leading `YYYY-MM-DD-` prefix if present, so an
   archived change's `Slug` is always the same value it had while active.
   `Change.Dir` keeps the real (date-prefixed) path — only the logical `Slug`
   identity is normalized.
3. **`resolveEntry`**: apply the same glob resolution as (1) for the
   `changes/archive/<slug>` candidate path.

## Out of Scope

- Changing `openspec`'s own archive-naming scheme (date-prefixing is that
  tool's convention, not something specsync controls).
- Auto-invoking `specsync` at the end of `openspec archive` — the two tools
  are intentionally separate (`openspec` is the planning-layer CLI, specsync
  is the tracker-sync layer); fixing slug resolution here is what makes the
  documented "sync once more, then archive" *or* "archive, then sync once
  more" order both actually work, without needing a hook between the tools.

## Release Notes

Fixed: `specsync -change <slug>` (and slug-based links.md entries) failed to
resolve a change after `openspec archive` renamed its folder with a date
prefix. Archived changes now resolve by their original slug, so a sync run
after archiving correctly projects the terminal `archived` stage to the
tracker and board instead of erroring with "no change found for slug".


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
