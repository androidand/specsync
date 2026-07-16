# Changelog

All notable changes to this project are documented here. One entry per shipped
OpenSpec change — see the linked issues for the full spec and discussion.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
