# Tasks

## Repo-stable ref-cache key
- [x] `GitHubProvider` resolves its concrete repo (from `-repo` or git-remote auto-detect) so `Name()` returns `"github:owner/repo"` consistently
- [x] Keep `repoFlag()` behavior unchanged for `gh` invocation (auto-detect still allowed on the wire)
- [x] Unit test: auto-detected and `-repo` providers for the same repo yield the same `Name()`

## Backward-compatible ref lookup
- [x] `syncOne` resolves the ref by canonical key, falling back to legacy bare `"github"`
- [x] Migrate a legacy-key hit to the canonical key on next save
- [x] Unit test: a `refs.json` with a bare `"github"` key updates the linked issue (no create) and is rewritten canonically

## Pull persists the identity marker
- [x] `pull` upserts `<!-- specsync:change=<slug> -->` into the source issue body (idempotent)
- [x] Honor `-dry-run` (preview the marker edit, no GitHub write)
- [x] Unit test: after pull + cache deletion, a sync rediscovers the issue via `Find` and updates it (no duplicate)

## Verification
- [x] `go build ./...` and `go test ./...` green
- [x] `gofmt` clean
- [ ] Manual: re-run the widget-app repro (pull-linked issue + auto-detect push) updates, not duplicates
