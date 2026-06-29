# Tasks: planning scan

## Area scope (consumes `add-change-traceability-model`)
- [ ] Confirm the foundation's `Scope` supports an area form (paths + optional topic); this change consumes it, does not redefine it
- [ ] Build an area `Scope` from `scan` arguments: zero or more path globs + optional topic; require at least one

## Resolution (over the trace engine)
- [ ] Paths → commits via `git log -- <globs>` → linked changes/issues/PRs through the trace graph
- [ ] Topic → text match over `openspec` change titles/proposals and open issue titles/bodies (`gh`/`openspec list`)
- [ ] Deterministic ranking: exact path matches, then topic matches, then recency; stable order for clean `--json` diffs
- [ ] Carry provenance on every result; never invent a connection

## Command (`cmd/specsync`)
- [ ] `scan` subcommand: `scan <area...>` with path and/or topic args, `--json`
- [ ] Human output grouped: In-flight changes (with status) / Open issues in area / Recently delivered
- [ ] `--json` output structured for a planning agent (changes+status, issues, commits, PRs, provenance)
- [ ] Graceful degradation: missing `openspec`/`gh` reported, not silently narrowed; `git` always available

## Boundaries & docs
- [ ] Read-only, deterministic, no LLM/inference; confirm no mutation path
- [ ] No code-symbol scanning and no `graph.json` (both deferred — see proposals)
- [ ] Keep `boundary_test.go` green (stdlib-only)
- [ ] Add `scan` to the specsync skill file: run it BEFORE authoring a proposal
- [ ] Self-test: `specsync scan` over a known area in this repo returns its in-flight changes and recent commits
