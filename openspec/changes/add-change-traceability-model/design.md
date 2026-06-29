# Design: change-traceability architecture

Shared foundation for `add-change-traceability-model` (this one) and
`add-release-traceability` (the read-only report built on it). Read it first.

## Philosophy: capture and reconstruct, never enforce

specsync serves AI-augmented work in a reality where specs and issues seldom
exist up front and evolve as the work happens. Its existing roadmap is uniform on
this: `living-plan` captures discoveries without demanding triage, preserves
original intent, and makes churn legible; `emergent-work-spinoff` promotes a
discovery to a scoped sibling; `two-way-reconcile` lets each layer be owned by the
right side. The through-line is **make structure cheap to add after the fact, and
reconcile gently — never block, never punish.**

This change obeys that line. It only *reconstructs* the link graph from evidence
that already exists; it adds no command, no gate, no policy. An earlier draft
included a validation/policy/hook layer (strict CI gates, exit-code contracts,
hook templates) — that was cut as contrary to the philosophy above. specsync
points out what's missing so you can backfill it cheaply; it does not fail your
build for it.

## The hard constraints (from the repo)

1. **Stdlib-only.** `boundary_test.go` fails the build on any non-stdlib import.
   Consequences: no git library (commits come from shelling `git log` behind an
   adapter), no commit-parser dependency (hand-rolled, minimal). Anything that
   needs parsing is hand-written or shelled out.
2. **Providers shell out to host CLIs** (`gh` today, `git` now). The core stays
   free of network/auth code; adapters are faked in tests by swapping the runner.
3. **Optional capabilities are type-asserted**, never widening the minimal core
   interface (the established `IssueReader` pattern). `CommitSource` follows it.

## Core domain vs. adapters

The core owns *meaning*; adapters own *I/O*.

| Concern | Lives in | Why |
|---|---|---|
| Trace graph + link resolution | core (`trace.go`) | pure data, deterministic, testable |
| Conventional Commit parsing | core (`commit.go`) | pure string parsing of a fixed grammar |
| Reading commits | adapter (`git.go`, shells `git log`) | I/O, host-specific |
| Reading the tracker | adapter (existing providers, `gh`) | I/O, host-specific |
| Reading OpenSpec changes/deltas/status | adapter (`openspec.go`, shells `openspec --json`) | OpenSpec owns the spec model; don't re-parse it |

**Why commit parsing is core, not an adapter:** the grammar is a fixed public spec
(Conventional Commits 1.0.0), needs no I/O, and must behave identically
everywhere. The *source* of commit text (Git) is the adapter. Parse minimally —
only what linking and the follow-up report need (type, breaking signal, issue/PR
refs); resist growing a full linter.

## OpenSpec is a host CLI, not a directory we parse

specsync shells out to three host CLIs now: `gh` (tracker), `git` (history), and
**`openspec`** (the spec model). OpenSpec 1.4.1 exposes machine-readable JSON —
verified against this repo:

- `openspec list --json` → changes with `completedTasks`/`totalTasks`/`status`.
- `openspec show <change> --json --deltas-only` → structured deltas with
  `operation: ADDED|MODIFIED|REMOVED`, spec name, requirement text, scenarios.
- `openspec validate --json` → validity, usable as a quality signal.

So the trace model sources its OpenSpec nodes, requirement **deltas**, and
completion status by calling `openspec`, exactly as it sources commits from
`git`. It does **not** re-implement spec/delta parsing — that is OpenSpec's job,
and duplicating it would drift from the tool that owns and validates the format.
This is the cleanest resolution of an earlier blind spot (treating `openspec/` as
a directory layout rather than a tool with its own contract).

Boundary note: the *existing* push/pull/link path still reads `proposal.md` /
`tasks.md` off disk — that projection code is unchanged and correct. Only the
*new* trace/scan/release features prefer the `openspec` CLI, because they need
validated deltas and status the raw files don't structure. Where the two notions
of "status" differ — specsync's `.status` convention (`researched`/`implemented`)
vs. OpenSpec's task-derived `in-progress`/`complete` — the trace features use
OpenSpec's, and the difference is reported, not reconciled.

If a future environment lacks the `openspec` binary, the adapter degrades to a
minimal on-disk read (best-effort, no delta operations); the CLI is the
authoritative path.

**"Authoritative" is version-scoped — treat the JSON as a contract.** The shape
(`deltas`, `operation`, `deltaCount`, `status`) is authoritative *for the
installed openspec version* and can drift across releases. The adapter therefore:
(1) pins a **minimum openspec version**, checked once via `openspec --version`,
and reports a clear error if older; (2) **parses tolerantly** — reads the fields
it needs, ignores unknown ones, and never hard-fails on an added field — exactly
how the GitHub provider treats `gh` JSON. A Node-side `openspec` upgrade must not
silently break the trace features.

**Spawn once, cache — don't invoke per change.** `openspec` is a Node CLI; a
process spawn per change is slow and it is now a hard runtime dependency on the
trace paths. The adapter calls `openspec list --json` once and `openspec show
--json --deltas-only` at most once per change *in scope*, memoizes results for the
invocation, and never spawns in a loop over all changes. This mirrors the
`refs.json` ethos: pay the I/O once, reuse it.

**Release impact is a join, not a lookup — flagged here, resolved in
`add-release-traceability`.** `openspec show --deltas-only` returns the deltas in
the *current working tree*, not as-of a git ref. A release signal is inherently
historical ("what requirements changed between v1.3 and v1.4"). So the release
feature is **OpenSpec deltas ⨯ git history** (which changes were
completed/archived in the range, via the commit source and archived-change
state) — not a single call at HEAD. The foundation deliberately supplies *both*
halves (the `OpenSpecSource` and the `CommitSource`); the join itself lives in the
consuming change, where its historical semantics are defined.

## The trace model

```
Change (OpenSpec)
  ├── WorkItem[]      (existing Ref — issues/cards)
  ├── PullRequest[]
  └── Commit[]        (parsed, minimally)
```

- `Commit`: Hash, Type, Scope, Description, Breaking (from `!` or `BREAKING CHANGE:`
  footer), IssueRefs[], PRRefs[], Author, Date, Raw, ConventionalOK.
- `Link`: a directed edge with a `Provenance` enum:
  `marker | branch | commit-footer | pr-body | ref-cache | links-md`. Provenance is
  reported so a human sees *why* two artifacts are linked.
- `Trace`: the resolved graph for a **scope**. The scope abstraction has three
  forms, because three consumers query the same engine from different angles:
  - **a change** (`--change <slug>`) — everything linked to one change;
  - **a revision range** (`--since <ref> [--until <ref>]`) — what the outbound
    follow-up report (`release-plan`) walks;
  - **an area** (paths and/or a topic string) — what the inbound planning query
    (`scan`) walks: "what already relates to here?"

  Resolution is best-effort and additive; a missing edge is a **reported gap**,
  never an error. Those gaps are the whole point — "this PR realized no spec" is
  the follow-up signal, and "nothing exists here yet" is the planning signal.

The resolver must therefore take a `Scope` value general enough for all three;
`scan` and `release-plan` are lenses on one engine, not separate machinery. (An
earlier draft scoped only by change-or-range; taking the planning query seriously
adds the area scope to the foundation so the consumers stay thin.)

This generalizes the two link mechanisms specsync already has — the issue-body
`specsync:change=<slug>` marker and `links.md` — into one provenance-tagged graph,
rather than inventing a parallel scheme. It is the substrate
`emergent-work-spinoff`'s typed links and `two-way-reconcile`'s layered ownership
both implicitly assume.

## Revision-range semantics

`since` defaults to the most recent reachable tag; `until` defaults to `HEAD`.
With no tag, `since` is the root commit. The Git adapter resolves these by
shelling `git describe`/`git log`; the core never assumes a tag exists.

## Configuration: deliberately none yet

This change ships no config file. Defaults only. When a real need for an override
appears (the follow-up report's bump mapping is the likely first), config will be
added as a small committed file at the **repo root** (`specsync.json`, JSON
because stdlib has no YAML parser) — never under the gitignored `openspec/`, since
target repos can't read it from there and CI wouldn't see it. Deferring it now
removes surface and the JSON-vs-YAML question entirely until it's earned.

## Deferred extensions (documented, not built)

The trace model makes several things *possible* that are deliberately out of
scope until earned:

- **`graph.json` export** — serialize the resolved graph for an external consumer.
  No present consumer needs it (`release-plan`/`scan` build the graph in memory),
  except possibly an LLM agent wanting one-shot context instead of many tool
  calls. Add it only when a real consumer materializes; it's a byproduct, not a
  headline.
- **Graphify-enrich (`spec↔code` edges)** — Graphify infers which symbols
  implement a requirement. That is *inferred*, probabilistic, and expensive; the
  trace graph is *asserted*, deterministic, and free. Never merge them. The only
  safe shape is an arm's-length, artifact-based path — e.g. a future
  `specsync enrich --from <graphify-artifact>` that writes spec→code links into
  `links.md` as **proposals for human/agent confirmation**, exactly like
  `links.md` is already writable and confirmable. Graphify's runtime (Python, LLM,
  vision) must never become a hard dependency. Optional, later, suggestions only.
- **Config + per-project bump overrides** — deferred as above (repo-root
  `specsync.json` when earned).

## Testing approach

- Parser: table-driven over Conventional Commits examples plus malformed inputs
  (`ConventionalOK=false`); the messy-message case is a first-class test, not an edge.
- Git adapter: faked runner returning canned `git log` output (mirrors the GitHub
  provider fakes), so tests need no real repo.
- Trace resolver: synthetic changes + canned commits/refs assert edges, provenance,
  and that unresolved relationships surface as gaps rather than fabricated links.
