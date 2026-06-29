# Design: planning scan

Builds on `add-change-traceability-model/design.md` (the trace engine, the
`Scope` abstraction, and the `openspec`/`git`/`gh` shell-out adapters). Covers
only what is specific to the inbound planning lens.

## The moment it serves

```
$ specsync scan src/integrations/ "integration modal"

Scan  src/integrations/  +  "integration modal"

In-flight changes
  cross-repo-linked-issues   #6   researched   touches src/integrations/modal.tsx
  living-plan                #2   in-progress  (topic match: "modal")

Open issues in area  (no linked change)
  #71  modal a11y pass
  #68  integration setup copy review

Recently delivered  (last 5 tags)
  PR #51  feat(ui): split integration modal     → improve-integration-modal (archived)
  3 commits touching src/integrations/ since v0.2.0

Nothing else here.
```

The job is to answer "what already exists here?" fast enough that an agent runs
it *before* writing a proposal, and trustworthy enough that it never invents a
connection. Every line traces to a real artifact via the trace graph's provenance.

## Why path/topic, not a code graph

At planning time you want "good-enough pointers fast," not a full semantic map.
The fast path is deterministic and free:

- **paths** → `git log -- <globs>` yields commits touching the area, which link
  (via the trace engine) to changes/issues/PRs;
- **topic** → case-insensitive substring over `openspec` change titles/proposals
  and open issue titles/bodies (via `gh search`/`openspec list`) — never fuzzy or
  semantic, so the result set is reproducible.

A semantic code-symbol graph (Graphify) is slower, token-expensive, and
non-deterministic — the wrong tool for a tight planning loop. It stays an
optional, arm's-length deep-dive (`enrich`, deferred), never on the `scan` path.
This keeps `scan` inside the stdlib-only, deterministic, single-binary invariant.

## Scope and ranking

`scan` constructs an **area** `Scope` (the third form the foundation defines) from
its arguments: zero or more path globs and an optional topic string. Either may be
empty; at least one is required. Results are ranked deterministically — exact path
matches first, then topic matches, then recency — with stable ordering so repeated
runs and `--json` output diff cleanly. No relevance scoring that depends on
non-deterministic input.

## Output

- Human summary by default, grouped as above (in-flight changes, area issues,
  recent delivery), each line carrying its identifier so it is clickable/linkable.
- `--json` returns the same slice structured for an agent: changes (with status
  from `openspec`), issues, commits, PRs, each with provenance. This is what lets a
  planning agent open a proposal with "Relates to #6 (in-flight), supersedes none,
  area last touched by PR #51."

## Boundaries

- **Read-only, deterministic, no inference.** Same posture as `trace`.
- **Surfaces gaps, does not fill them.** "Open issues with no linked change" is the
  planning signal; promoting one into a scoped change is `emergent-work-spinoff`
  (`specsync spinoff`). The natural seam — `scan` finds the gap, `spinoff` acts on
  it — is assistance, never enforcement, and lives in that change, not this one.
- **Degrades with its sources.** Missing `openspec` → changes read best-effort;
  missing `gh` → issues/PRs omitted with a note; `git` always present. A partial
  scan says what it couldn't reach rather than silently narrowing.
