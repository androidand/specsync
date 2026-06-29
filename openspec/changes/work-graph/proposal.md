# Work graph

> **Deferred (not active).** This expands specsync from "specs↔issues" to
> "specs↔issues↔commits↔PRs↔releases" — the graph-maintainer identity. That is a
> deliberate future direction, tied to the naming/identity shift we are *not*
> ready for yet. Kept here as a worked-out proposal so the thinking is preserved;
> revisit once specsync has clearly earned the sync identity. Until then, do not
> sync or build it.

The single question this answers: **"I'm about to plan work on X — what already
relates to it, and what state is that work in?"**

specsync already holds the authoritative core of that answer — `links.md` asserts
spec↔spec edges, and body markers + `refs.json` assert spec↔issue edges. What it
can't yet do is *join* that core with the git/gh facts that surround it (the PRs,
commits, and releases that touched the same work), so today an agent assembles
that picture by hand, differently every time. The valuable primitive is the
**deterministic join** across sources specsync already shells out to — not a new
data source, and not a new kind of edge.

## Principle: an asserted graph, never an inferred one

Every edge is *asserted* — authored in `links.md`/markers, or read as fact from
`git log` / `gh`. The graph is therefore deterministic, free to compute, and
rebuildable from ground truth. No probabilistic, LLM-derived, or heuristic edges
ever enter it. This is the line that keeps the graph trustworthy, and it is why
inference tools stay outside it (see Non-goals).

Invariants hold: standard-library only, single binary, shells out to `git` / `gh`
/ `openspec` through the existing runner abstraction so it stays testable.

## Solution

**An in-memory work graph** built on demand:

- **Nodes:** spec (change), issue, commit, PR, release.
- **Edges, each from an existing shell-out:**
  - spec↔spec — `links.md` (asserted)
  - spec↔issue — body marker / `refs.json` (asserted)
  - issue↔PR↔commit — `gh` native issue/PR linkage (search by issue number)
  - commit↔release — `gh release` / `git tag --contains`
  - work↔files — changed files from the linked PRs/commits

**`specsync relate <slug>`** (or `-path <file>`) builds the graph and prints the
slice connected to X: related specs with their issue + stage, and the PRs,
commits, and releases touching the same files — in stable, deterministic order.
It is **read-only** — it never mutates the tracker. This is the fast,
deterministic planning scan; it does only the join an agent does badly by hand,
not code search an agent already does well.

**Optional delta annotation.** When the OpenSpec CLI is on `PATH`, each spec node
is tagged with its requirement-delta summary (ADDED / MODIFIED / REMOVED counts).
When it is absent, the file-based baseline stands and the tag is omitted. Gated,
never required — mirroring how `gh` is treated.

## Non-goals

- **Serialized `graph.json` export** — no consumer exists yet; add it only when a
  real one does, not speculatively.
- **Graphify / any inferred or code-symbol edges** ("which function implements
  this requirement") — inference would pollute an asserted graph; deferred until
  there is a concrete consumer, and only ever as a separate referencing layer.
- **Release-plan report** — a future *query over this same graph*, where deeper
  OpenSpec delta consumption will land. Out of scope here so the join ships clean.
