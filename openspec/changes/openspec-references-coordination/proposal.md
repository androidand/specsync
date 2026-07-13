# Coordinate across repos via OpenSpec references and worksets

## Why

A backend repo and a frontend repo are often worked at the same time, in two
worktrees, on branches that depend on each other. The agent in one folder needs to
know the other exists, where it is on disk, and which of its changes/issues relate
— so it can compare and stay in sync.

OpenSpec 1.5.0 already provides the local half of this and we should **embrace it
rather than reinvent it**:
- **`references:`** in the committed `openspec/config.yaml` declares which sibling
  OpenSpec repos ("stores") this project depends on; `openspec context --json`
  resolves a working set (root + referenced stores) and surfaces the upstream spec
  index to agents.
- **`openspec workset`** is a machine-local named set of folders opened together
  (e.g. `front=…/frontend back=…/backend`) — the two-worktree ergonomics.

What is missing is the bridge to the tracker: the *local* reference graph and the
*GitHub* relationship graph are not kept in sync, and specsync's own planning
output is blind to referenced siblings. specsync owns that bridge.

## What Changes

- **Read OpenSpec coordination, don't duplicate it.** specsync reads
  `openspec context --json` (referenced stores + their resolved local paths) and
  `openspec workset list --json` (folder sets) — it adds **no** path registry of
  its own. Machine-local data stays in OpenSpec; nothing new is committed.
- **Surface referenced siblings in planning output.** `scan`/`relate` (and a new
  `--references` view) report, for the current repo, each referenced sibling repo,
  its local folder, and its related changes/issues — so an agent in the frontend
  worktree can locate and compare with the backend worktree.
- **Suggest, never auto-create, the tracker edge.** Where a reference implies a
  dependency, specsync *suggests* a `## Blocked by` entry for the user/agent to
  confirm; it does not silently write GitHub dependencies from a repo-level
  reference. The actual projection stays with the explicit typed-link sync
  (`issue-dependency-sync`, `epic-and-subissue-projection`). This keeps the
  "capture cheaply, reconcile gently, never enforce" line.

### Out of scope / explicitly deferred
- Projecting relationships to GitHub — that is the sibling changes' job; this one
  only reads OpenSpec coordination and surfaces/suggests
- Managing OpenSpec stores/worksets (creating, registering) — that is `openspec`'s
  job; specsync only reads them
- Following references more than one level deep (OpenSpec itself resolves one level)

## Capabilities

### New Capabilities
- `openspec-reference-coordination` — read OpenSpec `references:`/`workset` to make
  specsync's planning output aware of sibling repos and their local folders, and
  suggest the matching tracker edges without auto-creating them.

## Impact
- New code: read `openspec context --json` / `openspec workset list --json` (same
  CLI-as-source-of-truth, version-guarded, tolerant-parse discipline the trace
  features already use); surface siblings in `scan`/`relate`.
- Degrades cleanly when OpenSpec lacks references/worksets or the binary is older —
  the feature simply reports nothing extra. Stdlib-only; shells out.
