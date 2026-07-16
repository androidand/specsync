# Tasks

## Configuration (opt-in)
- [x] Add `-project owner/number` flag to `sync` and `pull` (mirrors `-repo`); parse org/user + number
- [x] Config fallback so the target project need not be retyped; unset = no board behavior (backward-compatible)
- [x] Add `-status-map "active=In Progress,archived=Done"` flag to `sync` and `pull` (fallback: `$SPECSYNC_STATUS_MAP`) wiring the configurable stage→Status mapping to the CLI; malformed input fails loud

## ProjectV2 plumbing (new `BoardProjector` capability)
- [x] Resolve the ProjectV2 node id from owner/number via `gh api graphql`
- [x] Resolve the Status single-select field id + option ids from the project schema; cache per run
- [x] Map stage → Status option name (configurable; default active→first non-terminal, archived→terminal); resolve name → option id
- [x] Match Status option names case-insensitively (stock boards use "In Progress"; the default is "In progress") and report the board's canonical casing
- [x] Detect membership: issue node id → project item id (present/absent)

## Projection on sync/pull
- [x] Ensure-on-board: add issue via `addProjectV2ItemById` when absent (idempotent, never duplicate)
- [x] Set Status via `updateProjectV2ItemFieldValue` — only when unset or specsync-managed (no clobber)
- [x] Set assignee to acting user (resolve viewer) or configured assignee; never overwrite existing assignees
- [x] Skip all board work entirely when no target project is configured

## Dry-run & safety
- [x] `-dry-run` prints the board plan (add item / set Status option / set assignee), no GraphQL writes
- [x] Never remove a board item or clear a human-set field

## Tests
- [x] Fake `gh` runner covers: project/field/option resolution, membership present vs absent, ensure-on-board idempotency, status/assignee non-clobber, dry-run makes no writes
- [x] Stock-cased board ("Todo" / "In Progress" / "Done"): default mapping resolves the In Progress option (no positional fallback to Todo), no rewrite when already correct, managed "Done" is overwritable
- [x] `-status-map` parsing: valid pairs, whitespace, env fallback, unknown stage / malformed pair / duplicate stage errors, mapping reaches `BoardTarget`

## Verification
- [x] `go build ./...`, `go test ./...`, `gofmt` clean
- [ ] Manual: sync a change with `-project org/6` → issue appears on the board, In progress, assigned; re-run is a no-op
