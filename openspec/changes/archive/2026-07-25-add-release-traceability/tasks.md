# Tasks: release traceability (follow-up report)

## SemVer (core, `semver.go`)
- [x] Parse `MAJOR.MINOR.PATCH[-pre][+build]`; compare; apply a bump (none/patch/minor/major)
- [x] Preserve pre-release/build on read, drop on a normal bump; tests

## Advisory release impact (core, `releaseimpact.go`)
- [x] Define `ReleaseImpact` (none|patch|minor|major) + `Reasons []string`
- [x] Map commit types (`feat`→minor, `fix`→patch, others→none); promote to major on `!`/`BREAKING CHANGE:`
- [x] Derive impact from OpenSpec requirement deltas (`REMOVED`→major, `ADDED`→minor, `MODIFIED`→patch) — the differentiating signal, sourced via the openspec CLI
- [x] **The join:** find contributing changes by walking git history in `[since,until]`; archive commit = the commit that moves a change out of `changes/`; read active-change deltas via `openspec show`, read archived-change delta headers (ADDED/MODIFIED/REMOVED) via `git show <archive>^:<path>` (the one direct-markdown read, since the CLI can't target a past ref) — do NOT treat HEAD's working-tree deltas as the whole signal
- [x] Pre-baseline semantics: with no accepted baseline, all deltas are `ADDED` (spec signal ≤ minor); note the missing baseline in output
- [x] Combine as the maximum across signals; attach a human reason per contributing signal
- [x] Fixed default mapping (no config overrides yet); tests per signal, the combination, the git-join, and the pre-baseline case

## Release-tool detection (adapter, `releasetool.go`)
- [x] Filesystem probes for release-please, changesets, release-it, semantic-release, standard-version, custom, none
- [x] Return a light descriptor (name, detected, evidence, responsibilities owned); never import or invoke the tool
- [x] Neutral reporting only — name the tool and what it owns, no editorializing on which tool is better; tests over fixture trees

## The report (core + `cmd/specsync`)
- [x] `trace` subcommand: raw resolved graph (`--change`, `--since`, `--until`, `--json`)
- [x] `release-plan` subcommand: Shipped / Loose ends / Archive candidates / Advisory bump / Release path sections (`--since`, `--until`, `--json`)
- [x] "Loose ends" from trace-graph gaps; "Archive candidates" from completed `tasks.md`/`.status`
- [x] Human-readable "what shipped" summary (no changelog file mutation)
- [x] `--apply` performs only spec archive actions; never Git/tracker/version/tag/release mutation
- [x] Per-package output when packages are configured (future); single block otherwise

## Boundaries & docs
- [x] Read-only by default; confirm no path mutates without `--apply`
- [x] Keep `boundary_test.go` green (stdlib-only)
- [x] Add `release-plan`/`trace` to the specsync skill file with examples
- [x] Self-test: run `release-plan` on this repo; confirm it detects goreleaser + git tags + GitHub notes and defers
