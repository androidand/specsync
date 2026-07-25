# Tasks: change-traceability model

## Conventional Commits parser (core, `commit.go`)
- [x] Define the `Commit` struct (Hash, Type, Scope, Description, Breaking, IssueRefs, PRRefs, Author, Date, Raw, ConventionalOK)
- [x] Parse the header `type(scope)!: description` per Conventional Commits 1.0.0
- [x] Detect breaking via `!` marker and via a `BREAKING CHANGE:`/`BREAKING-CHANGE:` footer
- [x] Extract issue refs (`#123`, `owner/repo#123`, `Closes/Fixes #n`) and PR refs from body/footers
- [x] Set `ConventionalOK=false` for non-conforming messages without erroring (the common case)
- [x] Keep it minimal — only what linking/reporting needs; do not grow a linter
- [x] Table-driven tests over spec examples + malformed inputs

## CommitSource adapter (`provider.go`, `git.go`)
- [x] Add `CommitSource` interface: `Commits(ctx, since, until string) ([]Commit, error)` (type-asserted, optional)
- [x] Implement a Git adapter that shells `git log` with a parseable `--pretty` format
- [x] Resolve default range: `since` = latest reachable tag, `until` = `HEAD`; root commit when no tag
- [x] Fake-runner tests with canned `git log` output (no real repo)
- [x] Keep `boundary_test.go` green (stdlib-only)

## OpenSpecSource adapter (`openspec.go`)
- [x] Define an `OpenSpecSource` interface for changes, deltas, and status (type-asserted, mirrors `CommitSource`)
- [x] Implement it by shelling `openspec list --json` and `openspec show <change> --json --deltas-only`
- [x] Map deltas to `{spec, operation: ADDED|MODIFIED|REMOVED, requirement}` for the release signal; do NOT re-parse spec markdown
- [x] Check `openspec --version` once against a pinned minimum; parse JSON tolerantly (ignore unknown fields, don't hard-fail) — treat the shape as a version-scoped contract like `gh` JSON
- [x] Spawn once and cache: `list` once, `show` at most once per in-scope change, memoized; never loop-spawn
- [x] Degrade gracefully when `openspec` is absent (minimal on-disk read, delta ops reported unavailable)
- [x] Reconcile the two status notions (specsync `.status` vs OpenSpec task-derived) by reporting, not merging
- [x] Fake-runner tests with canned `openspec` JSON (incl. an unknown-field case and a below-minimum-version case)

## Trace model (core, `trace.go`)
- [x] Define `TraceNode` kinds (Change, WorkItem, PullRequest, Commit) and `Link` with a `Provenance` enum (marker | branch | commit-footer | pr-body | ref-cache | links-md)
- [x] Define a `Scope` value covering all three: a change, a revision range, and an area (paths and/or topic)
- [x] Resolve change↔commit edges from commit issue/PR refs and the existing `specsync:change=` marker / `links.md`
- [x] Resolve change↔workitem edges from existing `Ref`s (reuse current cache/marker logic)
- [x] Resolve change/issue/commit/PR by area so `scan` and `release-plan` share the resolver: paths = shell globs via `git log -- <glob>`, topic = case-insensitive substring over change titles/proposals + issue titles/bodies; order exact-path, then topic, then recency (tie: commit date DESC, then slug)
- [x] Source PR nodes from resolved references (commit footer/branch, linked-issue refs); record `pr-body` provenance only when a body is actually read via `gh` (best-effort), else carry the reference's provenance
- [x] Report unresolved relationships as gaps; never fabricate a link
- [x] Tests: synthetic changes + canned commits/refs assert edges, provenance, gap reporting, and all three scopes

## Boundaries & docs
- [x] No CLI surface in this change (foundation only); confirm `go build ./...`, `go vet`, `go test ./...` pass
- [x] No config file; defaults only (see design.md — deferred until earned)
- [x] Update `doc.go` layering note to mention the trace model and commit source
- [x] Skill-file updates wait until the consuming report exists (in `add-release-traceability`)
