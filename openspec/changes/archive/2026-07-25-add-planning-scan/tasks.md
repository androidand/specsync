# Tasks: planning scan

## Area scope (consumes `add-change-traceability-model`)
- [x] Confirm the foundation's `Scope` supports an area form (paths + optional topic); this change consumes it, does not redefine it
- [x] Build an area `Scope` from `scan` arguments: zero or more path globs + optional topic; require at least one

## Resolution (over the trace engine)
- [x] Paths → commits via `git log -- <globs>` → linked changes/issues/PRs through the trace graph
- [x] Topic → case-insensitive substring over `openspec` change titles/proposals and open issue titles/bodies (`gh`/`openspec list`)
- [x] "Open issues in area, no linked change": in-area = title/body contains an area path or matches the topic, or referenced by a commit touching an area path; no-linked-change = no `specsync:change=` marker AND no trace edge to an in-flight change
- [x] Deterministic ranking: exact path matches, then topic matches, then recency; stable order for clean `--json` diffs
- [x] Carry provenance on every result; never invent a connection

## Command (`cmd/specsync`)
- [x] `scan` subcommand: `scan <area...>` with path and/or topic args, `--json`
- [x] Human output grouped: In-flight changes (with status) / Open issues in area / Recently delivered
- [x] `--json` output structured for a planning agent (changes+status, issues, commits, PRs, provenance)
- [x] Graceful degradation: missing `openspec`/`gh` reported, not silently narrowed; `git` always available

## Boundaries & docs
- [x] Read-only, deterministic, no LLM/inference; confirm no mutation path
- [x] No code-symbol scanning and no `graph.json` (both deferred — see proposals)
- [x] Keep `boundary_test.go` green (stdlib-only)
- [x] Add `scan` to the specsync skill file: run it BEFORE authoring a proposal
- [x] Self-test: `specsync scan` over a known area in this repo returns its in-flight changes and recent commits
