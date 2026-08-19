# Tasks

- [ ] 1. **Close the release gap first**: publish the current main (with
  `link-by-issue-reference`, 16/16 but npm still at 0.9.1 without it) so the
  shipped cross-repo linking actually reaches installed copies; archive
  `link-by-issue-reference` per the completion-hygiene rule. Add the
  publish-before-archive line to the release checklist.
  <!-- Deferred: outward-facing release-management step, done separately from the `epic` command itself. -->
- [x] 2. `epic` subcommand skeleton: parse `<title>`, `--repo` (optional,
  default auto-detect from git remote, same as `relate`/`link`), repeated
  `--child` via the existing `stringSlice` flag type (classify each as slug
  vs issue ref via `classifyArg` from `link.go`), `--dry-run`. (Flags use the
  double-dash form for this new command; existing commands are untouched.)
- [x] 3. Epic creation: `type:epic` + `specsync` labels (explicit
  `WorkItem.Labels`, bypassing the stage/priority default), roll-up body
  listing children; **idempotent** — reuse `Push`/`Find`/`marker()` unchanged
  by giving the epic `WorkItem.Slug: "epic:" + slugify(title)` (the existing
  title-to-slug normalizer in `pull.go`), so `Find` locates the existing epic
  by that slug's marker before creating (no new marker format, no local ref
  cache — see design.md).
- [x] 4. Child wiring, degraded mode: managed `## Related` upsert in both
  directions (epic body ↔ each child), reusing the shared renderer
  (`PushRelatedEdit`, extracted from `runLink`'s reference-edit sequence).
  Slug children are synced first if they have no ref.
- [x] 5. Child wiring, full mode seam: a type-asserted `SubIssueAttacher`
  capability check in `Epic()` — no provider implements it yet
  (`epic-and-subissue-projection` hasn't landed), so every run takes the
  degraded `## Related` path today; a future provider implementation slots in
  without changing the caller.
- [ ] 6. `--version` build info: distinguish a repo `dev` build from a
  published release (goreleaser ldflags), so "installed lags repo" is
  diagnosable in one command.
  <!-- Deferred: independent of the `epic` command mechanically (design.md); its own capability, `release-gap-guard`. -->
- [x] 7. Tests: idempotent re-run, mixed slug+ref children, cross-repo
  children, dry-run parity with real run (the dry-runner pattern) — see
  `epic_test.go`.
- [x] 8. Skill + README: document the epic workflow with the canonical
  scenario ("feature X needs Y in backend, Z in frontend"), including how it
  composes with `issue-dependency-sync` once that lands.
