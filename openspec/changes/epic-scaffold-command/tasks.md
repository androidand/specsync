# Tasks

- [x] 1. **Close the release gap first**: `link-by-issue-reference` archived
  (issue #18 closed, `spec:archived` added). The npm publish itself happens
  via the release tag cut right after this change is archived — the same
  tag that ships this change's own code — so both land in the same release.
  The "publish-before-archive checklist line" turned out to already exist
  (see task 6's correction): no separate checklist code was needed.
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
- [x] 6. `--version` build info: `versionString()` (`cmd/specsync/version.go`)
  reports the VCS revision (via `runtime/debug.ReadBuildInfo()`, no ldflags
  needed) for a local `dev` build, so it reads e.g. `dev (4075f69)` instead of
  a bare `dev` — told apart from both a released binary and another dev
  build.
  Correction from design.md: the other half of this task — "the release
  checklist grows one line: a capability shipped 16/16 MUST be published
  before its change is archived" — turned out to already exist and already be
  enforced in CI: `specsync release-plan -fail-on-archive-candidates` (in
  `.github/workflows/release.yml`, "Enforce OpenSpec archive hygiene", runs
  before every tag's goreleaser step) fails the release if a complete, shipped
  change is still sitting unarchived. No new checklist code was needed; this
  task only added the `--version` diagnostic.
- [x] 7. Tests: idempotent re-run, mixed slug+ref children, cross-repo
  children, dry-run parity with real run (the dry-runner pattern) — see
  `epic_test.go`.
- [x] 8. Skill + README: document the epic workflow with the canonical
  scenario ("feature X needs Y in backend, Z in frontend"), including how it
  composes with `issue-dependency-sync` once that lands.
