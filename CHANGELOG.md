# Changelog

All notable changes to this project are documented here. One entry per shipped
OpenSpec change — see the linked issues for the full spec and discussion.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.8.0] - 2026-07-16

### Added

- Workflow state and priority: `specsync changes` (with `-stage` filter and
  `-json`), `specsync set-stage <slug> <stage|auto>`, and
  `specsync set-priority <slug> <1-100|unset>`, backed by a committed
  `.specsync/metadata.json` per change. Stage derives with the precedence
  archived folder → metadata → legacy `.status` → task completion → default.
- Board reconciliation base state in a gitignored `.specsync/board.json`:
  a three-way merge (local stage vs. remote Status vs. last-synced base)
  detects human board moves and skips them instead of clobbering.

### Changed

- `Change` gained `Progress`, `Stage`, `StageSource`, and `Priority` fields
  (additive; changes without metadata behave as before). Library consumers
  that serialize `Change` should account for the new fields.
- Archived changes are immutable: `set-stage` and `set-priority` refuse them.

### Fixed

- `set-stage <slug> auto` and `set-priority <slug> unset` now clear only
  their own field instead of deleting the whole metadata file (an explicit
  priority survives unsetting the stage, and vice versa).
- `set-priority` no longer freezes a tasks- or `.status`-derived stage into
  an explicit metadata override as a side effect.
- `set-priority` now rejects archived changes, matching `set-stage`.

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
