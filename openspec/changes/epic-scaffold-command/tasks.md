# Tasks

- [ ] 1. **Close the release gap first**: publish the current main (with
  `link-by-issue-reference`, 16/16 but npm still at 0.9.1 without it) so the
  shipped cross-repo linking actually reaches installed copies; archive
  `link-by-issue-reference` per the completion-hygiene rule. Add the
  publish-before-archive line to the release checklist.
- [ ] 2. `epic` subcommand skeleton: parse `<title>`, `--repo` (optional,
  default auto-detect from git remote, same as `relate`/`link`), repeated
  `--child` via the existing `stringSlice` flag type (classify each as slug
  vs issue ref via `classifyArg` from `link.go`), `-dry-run`.
- [ ] 3. Epic creation: `type:epic` + `specsync` labels (explicit
  `WorkItem.Labels`, bypassing the stage/priority default), roll-up body
  listing children; **idempotent** — reuse `Push`/`Find`/`marker()` unchanged
  by giving the epic `WorkItem.Slug: "epic:" + normalizeTitle(title)`, so
  `Find` locates the existing epic by that slug's marker before creating (no
  new marker format, no local ref cache — see design.md).
- [ ] 4. Child wiring, degraded mode: managed `## Related` upsert in both
  directions (epic body ↔ each child), reusing the shared renderer. Slug
  children are synced first if they have no ref.
- [ ] 5. Child wiring, full mode: when `epic-and-subissue-projection` lands,
  attach children as native sub-issues and let the epic body roll up from
  `subIssuesSummary`; keep degraded mode as fallback for providers/tokens
  without the GraphQL scopes.
- [ ] 6. `--version` build info: distinguish a repo `dev` build from a
  published release (goreleaser ldflags), so "installed lags repo" is
  diagnosable in one command.
- [ ] 7. Tests: idempotent re-run, mixed slug+ref children, cross-repo
  children, dry-run parity with real run (the dry-runner pattern).
- [ ] 8. Skill + README: document the epic workflow with the canonical
  scenario ("feature X needs Y in backend, Z in frontend"), including how it
  composes with `issue-dependency-sync` once that lands.
