# Tasks: change-traceability model

## Conventional Commits parser (core, `commit.go`)
- [ ] Define the `Commit` struct (Hash, Type, Scope, Description, Breaking, IssueRefs, PRRefs, Author, Date, Raw, ConventionalOK)
- [ ] Parse the header `type(scope)!: description` per Conventional Commits 1.0.0
- [ ] Detect breaking via `!` marker and via a `BREAKING CHANGE:`/`BREAKING-CHANGE:` footer
- [ ] Extract issue refs (`#123`, `owner/repo#123`, `Closes/Fixes #n`) and PR refs from body/footers
- [ ] Set `ConventionalOK=false` for non-conforming messages without erroring (the common case)
- [ ] Keep it minimal — only what linking/reporting needs; do not grow a linter
- [ ] Table-driven tests over spec examples + malformed inputs

## CommitSource adapter (`provider.go`, `git.go`)
- [ ] Add `CommitSource` interface: `Commits(ctx, since, until string) ([]Commit, error)` (type-asserted, optional)
- [ ] Implement a Git adapter that shells `git log` with a parseable `--pretty` format
- [ ] Resolve default range: `since` = latest reachable tag, `until` = `HEAD`; root commit when no tag
- [ ] Fake-runner tests with canned `git log` output (no real repo)
- [ ] Keep `boundary_test.go` green (stdlib-only)

## OpenSpecSource adapter (`openspec.go`)
- [ ] Define an `OpenSpecSource` interface for changes, deltas, and status (type-asserted, mirrors `CommitSource`)
- [ ] Implement it by shelling `openspec list --json` and `openspec show <change> --json --deltas-only`
- [ ] Map deltas to `{spec, operation: ADDED|MODIFIED|REMOVED, requirement}` for the release signal; do NOT re-parse spec markdown
- [ ] Check `openspec --version` once against a pinned minimum; parse JSON tolerantly (ignore unknown fields, don't hard-fail) — treat the shape as a version-scoped contract like `gh` JSON
- [ ] Spawn once and cache: `list` once, `show` at most once per in-scope change, memoized; never loop-spawn
- [ ] Degrade gracefully when `openspec` is absent (minimal on-disk read, delta ops reported unavailable)
- [ ] Reconcile the two status notions (specsync `.status` vs OpenSpec task-derived) by reporting, not merging
- [ ] Fake-runner tests with canned `openspec` JSON (incl. an unknown-field case and a below-minimum-version case)

## Trace model (core, `trace.go`)
- [ ] Define `TraceNode` kinds (Change, WorkItem, PullRequest, Commit) and `Link` with a `Provenance` enum (marker | branch | commit-footer | pr-body | ref-cache | links-md)
- [ ] Define a `Scope` value covering all three: a change, a revision range, and an area (paths and/or topic)
- [ ] Resolve change↔commit edges from commit issue/PR refs and the existing `specsync:change=` marker / `links.md`
- [ ] Resolve change↔workitem edges from existing `Ref`s (reuse current cache/marker logic)
- [ ] Resolve change/issue/commit/PR by area (paths via git, topic via text match) so `scan` and `release-plan` share the resolver
- [ ] Report unresolved relationships as gaps; never fabricate a link
- [ ] Tests: synthetic changes + canned commits/refs assert edges, provenance, gap reporting, and all three scopes

## Boundaries & docs
- [ ] No CLI surface in this change (foundation only); confirm `go build ./...`, `go vet`, `go test ./...` pass
- [ ] No config file; defaults only (see design.md — deferred until earned)
- [ ] Update `doc.go` layering note to mention the trace model and commit source
- [ ] Skill-file updates wait until the consuming report exists (in `add-release-traceability`)
