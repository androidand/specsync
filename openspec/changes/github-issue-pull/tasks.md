# Tasks: Pull a GitHub issue into a local OpenSpec change

## 1. Provider read capability
- [x] 1.1 Define `FetchedItem` and the optional `IssueReader` interface in `provider.go`
- [x] 1.2 Implement `Get` on `GitHubProvider` via `gh issue view --json` (title, body, state, url, labels)

## 2. Pull engine
- [x] 2.1 Add `Pull`/`PullOptions` in `pull.go`, requiring a provider that implements `IssueReader`
- [x] 2.2 Resolve the slug: explicit flag, else existing body marker, else slugify the title
- [x] 2.3 Split the issue body: strip the marker, separate the `## Tasks` section into `tasks.md`
- [x] 2.4 Write `openspec/changes/<slug>/{proposal.md,tasks.md}`, ensuring proposal has an H1 title
- [x] 2.5 Cache the ref so a subsequent push updates the same issue (no duplicate)

## 3. CLI
- [x] 3.1 Add a `pull` subcommand: `specsync pull -issue <n> [-slug <s>] [-dry-run]`
- [x] 3.2 Keep default (no subcommand) behaviour as today's push/sync

## 4. Tests
- [x] 4.1 Pull creates a well-formed change from a faked issue (body + tasks split)
- [x] 4.2 Pull on an issue with no `## Tasks` writes proposal only
- [x] 4.3 Round-trip: pulled change pushes back to the same issue id (cache set)
- [x] 4.4 Dry-run pull writes nothing to disk

## 5. Docs
- [x] 5.1 Document the issue-first pull flow in the README
