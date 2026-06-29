# Change-traceability model: link specs to the work that realized them

## Why

specsync exists for a reality where specs, plans, and issues seldom exist up
front — they start vague or absent and take shape *as the work happens*,
especially when that work is AI-augmented and moves fast. The companion changes
already lean into this: `living-plan` captures discoveries cheaply, preserves
original intent, and makes plan churn legible; `emergent-work-spinoff` turns a
discovery into a scoped sibling; `two-way-reconcile` keeps each layer owned by
the right side. What none of them can do yet is connect a change to the **actual
work that realized it** — the commits and PRs. So the most common follow-up
question after a burst of agentic work — *"what did we actually build against
this spec, and what shipped with no spec at all?"* — can't be answered.

This change adds the connective tissue: a read-only model that links an OpenSpec
change to its issues (already known), its pull requests, and its commits. It is
the substrate the follow-up report (`add-release-traceability`) reads. It adds no
enforcement, no policy, no commands of its own — just the graph.

## What Changes

- Add a **trace model**: a provider-agnostic graph linking a `Change` to its
  `WorkItem`s (existing `Ref`s), `PullRequest`s, and `Commit`s. Every edge records
  a `Provenance` — *how* the link was found (issue-body marker, branch name,
  commit footer, PR body, ref cache, or `links.md`). Links are resolved from real
  evidence, never invented; an unresolved relationship is reported as a gap, which
  is exactly the "what has no spec" signal the consumers surface. The graph
  resolves over a **scope** that is a change, a revision range, *or an area*
  (paths/topic) — so one engine serves both the inbound planning query (`scan`,
  `add-planning-scan`) and the outbound report (`release-plan`).
- Add a **`CommitSource`** capability: an optional, type-asserted adapter (same
  pattern as `IssueReader`) that yields commits for a revision range. Ship a Git
  implementation that shells `git log` — no git library, honoring stdlib-only.
- Add an **`OpenSpecSource`** capability: source change metadata, requirement
  **deltas**, and completion status from the `openspec` CLI's JSON output
  (`list --json`, `show --json --deltas-only`), deferring to OpenSpec rather than
  re-parsing spec markdown. `openspec` becomes a third shell-out CLI beside `gh`
  and `git`.
- Add a **minimal Conventional Commits parser** in core: enough to extract the
  type, breaking marker/footer, and issue/PR references needed to link a commit to
  a change. Pure string parsing, no I/O, no dependency. It tolerates
  non-conventional messages rather than rejecting them — the messy middle is the
  normal case, not an error.

### Out of scope / explicitly deferred
- The follow-up report, bump inference, and release-tool detection (→ `add-release-traceability`)
- Any committed config file — defaults only for now; configuration is deferred until a feature demonstrably needs an override
- Validation, policy modes, CI gates, and hook integration — **dropped**: enforcement is contrary to specsync's "capture cheaply, reconcile gently" philosophy
- A full commit linter (commitlint's job) or a git library
- Any mutation of files, Git, or trackers — this slice is strictly read-only

## Capabilities

### New Capabilities
- `trace-model` — the resolved change↔issue↔PR↔commit graph with link provenance and reported gaps, scoped by change, range, or area.
- `commit-source` — an optional adapter that reads commits for a revision range (Git via `git log`).
- `openspec-source` — source OpenSpec change/delta/status data via the `openspec` CLI's JSON, deferring to it rather than re-parsing.
- `conventional-commits` — a minimal core parser for type, breaking signal, and issue/PR references.

## Impact

- New code (Go, stdlib-only): `trace.go` (model + resolver with a `Scope` for
  change/range/area), `commit.go` (parser), a `CommitSource` interface on
  `provider.go` with a Git adapter (`git.go`), and an `OpenSpecSource` adapter
  (`openspec.go`) shelling `openspec --json`.
- No change to the existing push/pull/link paths; `Sync`, `Pull`, `Link` and their
  tests are untouched. `boundary_test.go` (stdlib-only) stays green.
- Generalizes the two link mechanisms specsync already has (the issue marker and
  `links.md`) into one provenance-tagged graph, rather than a parallel scheme — so
  it sits *under* `emergent-work-spinoff` and `two-way-reconcile`, not beside them.
