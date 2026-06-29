# Beads memory bridge

[Beads](https://github.com/gastownhall/beads) is a dependency-graph issue tracker
built as *persistent memory for coding agents*: hash-id issues with typed graph
links (`relates_to`, `supersedes`, `duplicates`, parent-child), `bd ready`
(unblocked work), `bd remember` / `bd prime` (memory injected at session start),
and "memory decay" that compacts closed tasks into summaries.

OpenSpec and Beads overlap enough that running both by hand is hard to justify —
both hold "tasks." But their purposes differ: OpenSpec authors **intent**
(behavioral, human-reviewed, front of the pipe); Beads is **long-term execution
memory** an agent queries (back of the pipe). specsync already sits between them,
so it can own the transition and remove the double-entry instead of adding a
third tool to maintain.

The convergence worth naming: Beads' *memory decay* and this repo's
*archive-retention* gate are the same idea — compact finished work into durable,
queryable memory. specsync should emit that memory; Beads is one place to put it.

## Principle: optional complement, never a requirement

- Core specsync stays standard-library-only with zero Beads coupling.
- Integration activates **only** when `bd` is on PATH or a `.beads/` dir exists;
  otherwise every command behaves exactly as today.
- Talk to Beads through its CLI / plain `issues.jsonl` interchange — never its
  Dolt internals — mirroring specsync's existing "file-based baseline" stance.

## Solution

**Memory sink on archive (primary).** Hook the `archive` event from
`archive-retention-lifecycle`. When Beads is present, emit a compacted memory
record for the closed change — title, the "why", final scope, key decisions, and
the issue URL — via `bd remember` (or a `type:message` memory bead). Result: when
`prune` retention removes the local folder, the intent survives in *two* durable
forms — the closed tracker issue (human-facing) and Beads memory (agent-facing,
surfaced by `bd prime`).

**Typed links (adjacent win).** Extend `specsync link` with
`-type relates|supersedes|duplicates|blocks|parent` (stolen from Beads' relation
set). Render the type into the issue `## Related` section, and when Beads is
present project the edge with `bd dep add`. Turns the current flat "Related" list
into a real graph specsync and Beads share.

## Non-goals

- Replacing the GitHub provider or making Beads the default sink.
- Coupling to Dolt, or to Beads' server mode.
- Atomic multi-agent `--claim` and runtime task orchestration — that is Beads'
  job at execution time, not specsync's at sync time.
- Projecting every task as a bead with full `bd ready` semantics — promising, but
  earns its own change once the memory sink proves the integration shape.
