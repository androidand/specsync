# Release traceability: a read-only follow-up report

## Why

After a burst of AI-augmented work — specs that started vague, issues filed
mid-stream, commits and PRs landing fast — the hardest thing is *following up*.
What did we actually build? Which specs did it realize, and which work landed
with no spec at all? Which changes are effectively done and should be archived?
This is the organizing work teams say they always *needed* to do and always
*failed* to do, because reconstructing it by hand is tedious and nobody does it.

specsync already holds the pieces: the trace graph from
`add-change-traceability-model` knows how changes, issues, PRs, and commits
connect. This change turns that graph into a **single read-only report** that
answers "what happened and what's loose" — so follow-up becomes one command, not
an archaeology project.

It is a *report*, not a release tool. It never bumps versions, writes tags,
publishes, or owns a changelog — it detects the release tool you already use and
defers to it, adding only a recommendation and the evidence behind it.

## What Changes

- Add a **`release-plan`** command (and the lower-level **`trace`** command) that
  reports, for a revision range: the OpenSpec changes included, the issues and PRs
  linked, the commits that contributed, **work that links to no spec** (the loose
  ends), **changes whose tasks are all done** (archive candidates), and a
  human-readable summary of what shipped. Read-only by default.
- Add an **advisory SemVer signal**: a recommended bump (`none`/`patch`/`minor`/
  `major`) inferred from Conventional Commit types, breaking markers/footers, and
  — the part no other tool can see — **OpenSpec requirement deltas** in a change's
  `specs/`. Every recommendation carries its reasons. It is a suggestion the
  report prints; the release tool still owns the actual bump.
- Add **light release-tool detection**: probe for the common tools (release-please,
  changesets, release-it, semantic-release, standard-version, or none) and report
  which one owns bumping/tagging/changelog/publishing, so specsync visibly stays in
  its lane. Detection reads the filesystem and never invokes the tool.
- Add one **opt-in mutation**: `--apply` performs only specsync-owned spec actions
  the report suggested — archiving a completed change. It never touches Git, the
  tracker's issues, versions, tags, or releases.

### Out of scope / explicitly deferred
- Bumping versions, writing tags, publishing, cutting GitHub releases — owned by the detected release tool, always
- Owning or writing a changelog — the report *summarizes* what happened; it does not mutate `CHANGELOG.md` (a future opt-in could, if demanded)
- Validation, policy modes, CI gates, hook integration — **dropped**: enforcement is contrary to specsync's "capture cheaply, reconcile gently" philosophy
- A committed config file and per-project bump overrides — deferred until a real need appears (see `add-change-traceability-model` design)
- Deep spec↔reality reconciliation — that is `two-way-reconcile` / `living-plan`; this report only *surfaces* drift and loose ends, it does not resolve them

## Capabilities

### New Capabilities
- `release-plan` — a read-only follow-up report: what shipped, what it links to, what's loose, what's archivable, plus an advisory bump.
- `release-impact` — infer an advisory SemVer bump from commit types, breaking signals, and OpenSpec requirement deltas, with reasons.
- `release-tool-detection` — detect the project's release tool and report its responsibilities, deferring to it; never invoke it.

## Impact

- New code (Go, stdlib-only): `semver.go` (minimal parse/compare/bump),
  `releaseimpact.go`, `releasetool.go` (filesystem probes), and `release-plan`/
  `trace` subcommands in `cmd/specsync`. Consumes the trace model and commit source
  from `add-change-traceability-model`; adds no tracker mutation.
- After this lands, the specsync skill file gains `release-plan`/`trace` so agents
  can produce a follow-up report at the end of a work session.
- Self-test: run `release-plan` on this repo — it should report goreleaser + manual
  git tags as the release path, GitHub-generated notes as the changelog, and defer
  accordingly.
