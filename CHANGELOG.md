# Changelog

All notable changes to this project are documented here. One entry per shipped
OpenSpec change — see the linked issues for the full spec and discussion.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.12.0] - 2026-08-19

### Added

- `specsync doctor` now checks that the `openspec` binary is installed and reachable, reporting its path/version or a fix recommendation when missing; `doctor install --json` now also emits structured JSON. (#132)
- Idea intake command (#101)
- `specsync epic <title> --repo owner/name --child ...` mints a `type:epic` coordination issue and wires cross-repo children to it — local change slugs or existing issue references — idempotently. (#100)

<!-- 8 internal commit(s) omitted (chore/docs/ci/...) -->

## [0.11.2] - 2026-08-13

### Added

- Idea intake command (#101)

<!-- 2 internal commit(s) omitted (chore/docs/ci/...) -->

## [0.11.1] - 2026-08-10

### Fixed

- archive closeAndLabel pushed a blank-title WorkItem and never attached the label (1462356e)
- retry Find before create to close a duplicate-issue race (7cd024dd)

<!-- 1 internal commit(s) omitted (chore/docs/ci/...) -->

## [0.11.0] - 2026-08-10

### Added

- add OpenSpec coordination support (references, worksets) (03a0dc5d)
- add archive lifecycle, board two-way merge, and MCP provider type (24dcc5a7)
- add LabelApplier interface and idea/ideas commands (520087f3)

### Fixed

- resync derived SKILL.md copies with the canonical minimal version (7a4ae8e5)
- set GH_TOKEN so gh-calling CI steps can authenticate (b40fad91)

<!-- 3 internal commit(s) omitted (chore/docs/ci/...) -->

## [0.10.1] - 2026-08-10

### Added

- auto-update skills on binary upgrade (#120) (#121) (08801273)

<!-- 2 internal commit(s) omitted (chore/docs/ci/...) -->

## [0.10.0] - 2026-08-10

### Added

- Added `specsync doctor` command for diagnosing and optimizing Claude Code token usage. Run `specsync doctor` to see installation status, detect duplicate skills, and get personalized recommendations. Use `specsync doctor --json` for automation and scripting. (#114)
- Advisory title suggestions: warn on unwieldy titles, never rewrite them (#52)
- Beads auto-detection requires a Beads database (#103)
- Board state reconciliation: three-way merge for shared workflow (#42)
- Add `specsync validate` CI gate (#64)
- directed dependency links (BlockedBy/Blocks) with GitHub dependency projection (2847f7be)
- add idea-intake-command spec; ignore .status files (2d46a1b0)
- PR-issue traceability with pr-body and verify subcommands (#??) (2e676aa7)
- correct archived import path record and add guard test (47b9dcf4)
- add audit-archived-not-merged change (5ba49132)
- add set-stage/set-priority tests and atomic write verification (#58) (6664afd9)
- add sorting, TASKS/TITLE columns, stage grouping to specsync changes (71225035)
- pluggable SpecSource interface with FileSpecSource and BeadsSource (844f7fee)
- reduce SKILL.md token cost to 60% (#12) (a4213098)
- add specsync audit command and shipped stage (#58) (d815df3d)
- pluggable SpecSource interface with FileSpecSource and BeadsSource (ecaf195e)
- surface external-close deferral in run output (f311c359)

### Changed

- Consolidated agent documentation to reduce duplication. AGENTS.md is now the authoritative agent guide; CLAUDE.md and .claude/CLAUDE.md are lightweight entry points that reference it. No content lost; improved organization and maintainability. (#117)
- Epic & sub-issue projection (#2)
- Linker resolves issue refs with source visibility (branch, cache) and slug-aware branch matching to prevent cross-change resolution. (#96)

### Fixed

- correct archived record, document import path, add guard test (correct-library-import-path) (000ece41)
- record the task merge base, don't drop it (0251e239)
- sync .claude SKILL.md with canonical (037202f5)
- require a .beads/ database to auto-select the Beads provider (09f1d6e4)
- require a project Beads database for auto-detection (#102) (0c9a635e)
- restore AuditTasks to audit.go; fix parseLinksMD 3-return-value call (4c4dd691)
- audit command syntax error, branch matching, archived immutability (5330792a)
- make links.md append-only, never rewritten (575f7dfe)
- build errors in traceability.go and make loadRefs/TasksComplete public (5a134148)
- remove merge conflict markers from changes_test.go (8ad300dc)
- only reverse a close specsync made itself (c5969271)
- two defects found reviewing the preceding three commits (c9e92751)
- remove relate case from main.go (belongs to work-graph change) (d07979d0)
- require a .beads/ database to auto-select the Beads provider (e5a0c9ca)
- repair CI workflow YAML indentation bug (f267b725)

<!-- 16 internal commit(s) omitted (chore/docs/ci/...) -->

## [0.9.1] - 2026-07-27

### Added

- **Repo resolution is now explicit.** specsync resolves the target repository deterministically and always passes `--repo` to every `gh` invocation. Resolution order: `-repo` flag → `gh repo set-default` → `origin`. Fork-parent writes are refused by default; use `-repo` to override. Board resolution also gained a repository-local declaration: `openspec/specsync.yml` with `board: owner/number`. The `-project` flag and this config file are the only ways to set a board; the `SPECSYNC_PROJECT` env var is no longer used. (#76)
- Multi-provider sync (fan-out) (#24)
- Auto-detect Beads provider (#62)
- Link existing issues by reference, without scaffolding specs (#18)
- fix task dogfooding — audit-tasks command and reconcile all changes (#61) (03f09859)
- convergence check for 3-way board merge (#42) (#74) (04ea25a6)
- linker cleanup — LinkerResult, slug-matching, dead code removal (#97) (079b9132)
- spinoff subcommand — spawn linked child changes from discoveries (#9) (0c2bd80f)
- add --worktree flag to pull — create or reuse worktree (#51) (#72) (18c09518)
- add specsync validate command for change structure (#64) (#67) (22cfa581)
- spec-issue-linker — pull integration, Linker context resolution, skill doc (#95) (47cedb84)
- spec-issue-linker — Linker interface, resolvers, sync integration (8b752045)
- stable task ID — position-based mapping for rewritten task detection (#65) (#73) (b542d927)
- slug validation, archived priority, slug tests (#43) (b6f71361)
- slug validation, archived priority support (#43) (#75) (f0d8d40e)

### Changed

- Claim work in flight so concurrent agents don't collide (#78)
- Work graph (#19)

### Fixed

- Add `specsync audit` — a read-only command that cross-references archived OpenSpec changes against GitHub PRs to find archived changes whose PR was never merged. Also add a new `shipped` stage that represents the final step in the lifecycle: the PR has landed. (#59)
- apply changelog 0.9.1, bump npm to 0.9.1, archive linker-cleanup (#98) (ee000c8a)

<!-- 19 internal commit(s) omitted (chore/docs/ci/...) -->

## [0.9.0] - 2026-07-17

### Added

- Advisory title suggestions: warn on unwieldy titles, never rewrite them (#52)
- The changelog now ignores commits that were reverted within the same release range, so net no-op work no longer produces entries describing behavior the release doesn't contain. (#53)
- Fail CI when a commit ships without a linked issue (#51)

### Fixed

- don't make push an alias — point it at sync instead (46f5478c)
- reject unrecognized leading arguments instead of silently falling through to sync (5893ebef)

<!-- 21 internal commit(s) omitted (chore/docs/ci/...) -->

## [0.8.0] - 2026-07-16

### Added

- add three-phase workflow state management for SpecSync (0ba30bd4)
- phase 1 - implement SpecSource interface and OpenSpec implementation (1065b593)
- add SpecSourceFactory and --spec CLI flag (1094ca19)
- implement rich-change-state foundation (1ae74b8a)
- add change-status commands (partial) (36c7ddeb)
- implement three-way merge reconciliation with human-move detection (4b7bc5a2)
- add three-way merge infrastructure for board-state-reconciliation (8386fe9c)
- add explicit archive-completed execution (83898732)
- rework changelog, hero, IA, and a11y per review feedback (d0de2e57)
- complete change-status-cli with atomic writes and JSON output (fe6e6a39)

### Changed

- **Breaking:** repair set-stage/set-priority semantics; drop dead code and artifact docs (3cd81b7f)

### Fixed

- show unlinked-but-shipped commits too, not just spec-backed ones (07602bd3)

<!-- 15 internal commit(s) omitted (chore/docs/ci/...) -->

## [0.7.0] - 2026-07-14

### Added

- resolve refs live at release time, wire into release CI (d0578292)
- render changelog from CHANGELOG.md, not the GitHub API (e2c18f8d)

### Fixed

- never let a failed build degrade committed content (9035465c)
- stop reading bare #N in commit prose as issue evidence (aeee62a9)

<!-- 1 internal commit(s) omitted (chore/docs/ci/...) -->

## [0.6.0] - 2026-07-14

### Added

- GitHub Projects (board) projection: opt-in `-project owner/number` (or
  `$SPECSYNC_PROJECT`) syncs an issue onto a GitHub Projects v2 board, maps
  the change's stage to the board's Status field, and assigns the acting
  viewer — unconfigured stays a complete no-op, zero board calls. (#37)
- `specsync changelog`: a Keep a Changelog section built from shipped OpenSpec
  changes via the trace graph — one entry per change, release notes authored
  at planning time, never a raw commit dump.

### Fixed

- Two-way sync no longer duplicates a GitHub issue after `pull` — the
  ref-cache key is now repo-stable, and a legacy pre-fix cache entry can no
  longer be mistaken for a different repo's issue and edited by accident. (#35)
- Board Status option names now resolve case-insensitively (a stock "To do /
  In Progress / Done" board no longer falls back to "Todo" for active work),
  and the promised stage→Status mapping is reachable via `-status-map` (or
  `$SPECSYNC_STATUS_MAP`).
