# Tasks

## 1. Core: PR state reader

- [ ] 1.1 Add `ListOpenPRs(ctx) ([]PRState, error)` to `GitHubProvider` that calls `gh pr list --state open --json number,url,title,headRefName,body`.
- [ ] 1.2 Add `ListRecentMergedPRs(ctx) ([]PRState, error)` to `GitHubProvider` for `gh pr list --state merged --limit 50 --json number,url,title,headRefName,body`.
- [ ] 1.3 Add `PRState` type with fields: Number, URL, Title, HeadRefName, Body, Merged (bool).
- [ ] 1.4 Add unit tests for `ListOpenPRs` and `ListRecentMergedPRs` using `NewGitHubProviderFunc`.

## 2. Matching: PR ↔ change

- [ ] 2.1 Write `matchPRToChange(pr PRState, slug string) bool` — matches slug against branch name prefix, PR title, and specsync marker in body.
- [ ] 2.2 Add unit tests for matching logic with realistic PR data.

## 3. Audit command

- [ ] 3.1 Add `Audit` function in `audit.go` that loads archived changes, queries open + recent merged PRs, and classifies each change as unmerged/shipped/orphaned.
- [ ] 3.2 Add `AuditResult` type: `[]AuditFinding` with fields Change, Slug, PR, Status (unmerged/shipped/orphaned).
- [ ] 3.3 Add `specsync audit` subcommand in `cmd/specsync/main.go` with table output.
- [ ] 3.4 Add `-json` flag to `specsync audit` for machine-readable output.
- [ ] 3.5 Add `-repo` flag to `specsync audit` (reuse existing flag pattern).
- [ ] 3.6 Add `-fail-on-unmerged` flag to `specsync audit` — exits non-zero when unmerged changes exist.
- [ ] 3.7 Add unit tests for `Audit` using mock GitHub provider.

## 4. Shipped stage

- [ ] 4.1 Add `StageShipped` to canonical stages in `change.go`.
- [ ] 4.2 Extend `ValidateStage` and `IsCanonicalStage` for `shipped`.
- [ ] 4.3 Update `CanonicalStageOrder` to include `shipped` after `archived`.
- [ ] 4.4 When `specsync audit` finds a merged PR for an archived change, write `stage: shipped` to `.specsync/metadata.json` (with `-mark-shipped` flag, opt-in).
- [ ] 4.5 Add `-mark-shipped` flag to `specsync audit` — writes metadata for confirmed merges.
- [ ] 4.6 Add unit tests for shipped stage derivation and metadata write.

## 5. Verification

- [ ] 5.1 Run `go test ./...` — all tests pass.
- [ ] 5.2 Manual test: run `specsync audit` against a repo with known archived+unmerged changes.
