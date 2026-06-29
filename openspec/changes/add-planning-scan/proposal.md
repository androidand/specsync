# Planning scan: "what already exists here?" before you plan

## Why

specsync exists because specs, plans, and issues seldom exist up front — they
start vague and take shape as the work happens. The single highest-leverage
moment in that reality is **the start of planning**: you're about to work on
something, and the question that decides whether structure gets reconstructed or
lost is *"what already exists here — specs, issues, recent work — that I should
build on instead of duplicating?"* Today answering it means manually grepping
`openspec/`, scrolling issues, and reading `git log`. So it usually doesn't
happen, and work gets re-planned from scratch or collides with work in flight.

This is the **inbound** twin of `release-plan`. Where `release-plan` looks back
("what shipped, what's loose"), `scan` looks at the present from a topic/area
("what relates to here, right now"). Both are lenses on the one trace engine from
`add-change-traceability-model`; this change adds the inbound lens and its
command. It is the front of the funnel, and the more on-vision of the two
consumers — so it is sequenced first.

## What Changes

- Add a **`scan`** command: `specsync scan <area>` resolves the trace graph for an
  *area* scope (paths and/or a topic string) and reports the relevant slice for
  planning — in-flight OpenSpec changes on the topic with their status, open
  issues in the area with no linked change, and recent commits/PRs/releases that
  touched the same files. Read-only, fast, deterministic.
- Source every input from what specsync already shells out to: `openspec --json`
  (changes/status), `git log` (commits/paths), `gh` (issues/PRs). **No LLM, no
  code-graph, no Python** — "good-enough pointers fast" beats a slow semantic
  graph at planning time.
- Output a human summary by default and `--json` for a planning agent to consume
  directly, so an agent writing a proposal can open with what already exists.

### Out of scope / explicitly deferred
- Code-symbol scanning ("which functions implement this") — that is the deferred, arm's-length Graphify-enrich path, not the fast planning loop; `scan` stays at path/topic granularity
- Emitting a standalone `graph.json` — deferred until a real external consumer exists (see `add-change-traceability-model` design)
- Any mutation, and any LLM/inference — `scan` is deterministic and read-only
- Scaffolding the missing change a scan reveals — that is `emergent-work-spinoff` (`specsync spinoff`); `scan` surfaces the gap, spinoff acts on it

## Capabilities

### New Capabilities
- `planning-scan` — a read-only, deterministic `scan <area>` that returns the planning-relevant slice of the trace graph (in-flight changes, area issues, recent delivery) from `openspec`/`git`/`gh`.

## Impact

- New code (Go, stdlib-only): a `scan` subcommand in `cmd/specsync` plus an
  area-scope query over the trace resolver. Consumes the `Scope`/area resolution,
  `CommitSource`, and `OpenSpecSource` from `add-change-traceability-model`; adds
  no new infrastructure and no tracker mutation.
- After this lands, the specsync skill file gains `scan` so a planning agent runs
  it *before* authoring a proposal — turning "the spec didn't exist" into "here's
  what already does."
- Reinforces the product identity surfaced in the rename discussion: specsync
  maintains the planning↔delivery graph and serves it **both directions** —
  inbound (`scan`) and outbound (`release-plan`).
