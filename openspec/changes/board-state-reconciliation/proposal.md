# Board state reconciliation: three-way merge for shared workflow

## Why

GitHub Projects users can move cards between statuses, but specsync's current board projection is one-way: it updates the board to match local state but does not read back human moves. A human moving a card to "Done" gets silently moved back to "In progress" on the next sync if local tasks are incomplete.

This breaks collaboration. The board is part of the shared workflow; human curation should not be trampled.

But bidirectional sync is dangerous without explicit merge semantics. Without knowing what specsync last wrote, it cannot tell whether a board status change is a human intention ("I'm done") or drift that should not be imported.

This change implements three-way merge: local stage, remote status, and the exact last-written value form a merge base. Conflicts (both sides changed) are detected and reported; safe changes (only one side moved) are imported. Non-conflicting human moves feed back into local `.specsync.yaml` stage.

## What Changes

### Board Binding Storage

`.specsync/board.json` (gitignored, multi-provider):

```json
{
  "version": 1,
  "bindings": {
    "github-projects:owner/5": {
      "provider": "github-projects",
      "project_id": "PVT_kwDOBx1M-s4Ae5nM",
      "item_id": "PVTI_lADOBx1M-s4Ae5nMzgABCDE",
      "base": {
        "local_stage": "active",
        "remote_status_option_id": "47fc9ee4"
      },
      "synced_at": "2026-07-15T14:30:00Z"
    }
  }
}
```

Schema supports multiple providers and multiple projects per change (future: Beads, Linear, etc.).

### Three-Way Merge Algorithm

At sync time:

1. Read local stage from `.specsync.yaml` or derived
2. Read remote status from GitHub Projects API (by option ID)
3. Read last-written base from `.specsync/board.json`

Compare:

```go
localChanged := currentLocalStage != base.LocalStage
remoteChanged := currentRemoteOptionID != base.RemoteOptionID

switch {
case !localChanged && !remoteChanged:
    // Nothing changed; no action

case localChanged && !remoteChanged:
    // Local changed, remote stable; push local stage

case !localChanged && remoteChanged:
    // Remote changed, local stable; pull remote status if unmapped unambiguously

case localChanged && remoteChanged:
    if mappedLocalStage == currentRemoteOptionID {
        // Both converged independently to same value
    } else {
        // Genuine conflict; report, do not auto-resolve
    }
}
```

### Reverse Mapping Rules

Mapping a remote status back to local stage requires care: multiple stages may map to the same status.

Define a reverse mapping policy:

**Option 1: One-way projection (Phase 1, recommended)**
- Push local stage → remote status
- Do not read remote status back
- Preserve human board moves (do not trample) but do not import them as stage changes
- Safest first release; validates the model

**Option 2: Strict bidirectional (Phase 2, optional)**
- Require -status-map to be unambiguous in both directions
- Example:
  ```bash
  specsync sync -project owner/5 \
    -status-map "backlog=Todo,blocked=Blocked,active=In Progress,in-review=Review,complete=Done"
  ```
- Check that no two stages map to the same status; error if ambiguous
- Implement reverse mapping (Todo → backlog, Blocked → blocked, etc.)
- Import human board moves as stage changes

**Option 3: Explicit import mapping (Phase 2, alternative)**
- Separate -status-map and -status-import-map
- Export: backlog → Todo, active → "In Progress"
- Import: Todo → backlog, "In Progress" → active
- More verbose but fully explicit

### Archived Item Behavior

When an archived change was previously projected to a board:

1. Do not remove the item from the board (destructive)
2. Set its status to "Done" (or equivalent terminal status)
3. Optionally archive the Project item (if GitHub's API supports and flag is set; future enhancement: `-archive-project-items`)

### New Capabilities

- `board-state-binding`: per-provider, multi-project binding storage
- `three-way-merge`: detect local/remote/base changes, report conflicts
- `conflict-detection`: distinguish safe imports from conflicts
- `status-mapping`: extend existing -status-map syntax for new stages
- `remotevalue-query`: read exact remote status option IDs and update times

### Integration with Existing Features

- Works with new Stage enum and six canonical stages
- Backward compatible with existing one-way projections
- No breaking changes to -project or -status-map flags

### Out of Scope

- Personal vs team board state (future: worksets)
- Automatic conflict resolution (human decision)
- Blocking relationships or dependency tracking
- Custom field projections beyond Status

## Impact

**Code Changes**:
- `.specsync/board.json` format and persistence
- Three-way merge algorithm
- Remote query extensions (exact option ID and update timestamp)
- Conflict reporting in sync output
- New flag: `-conflict-mode` (one-way, strict-bidirectional, explicit-mapping) — defaults to one-way for Phase 1

**Compatibility**:
- Existing `-project` and `-status-map` flags continue to work
- One-way projection is default; bidirectional is opt-in (future phase)
- No changes to issue or change metadata

**Breaking Changes**: None for Phase 1 (one-way, preserving human moves).

## Implementation Phases

**Phase 1: One-way projection with human-move preservation**
- Implement board.json binding storage
- Query remote status and last-written value
- Detect human moves (remote != base) and report
- Do not import; do not trample
- Safe, reviewable first release

**Phase 2: Strict bidirectional (future)**
- Implement three-way merge
- Require unambiguous -status-map
- Implement reverse mapping
- Import safe remote changes as local stage

**Phase 3: Explicit mapping (future alternative)**
- Add -status-import-map flag
- Support multiple mapping strategies

## Dependencies

- Depends on `rich-change-state` for Stage enum and `.specsync.yaml`
- Depends on `change-status-cli` for table/JSON output of stage information
- Compatible with `board-status-two-way` (complementary, not overlapping)

## Recommended Workflow

1. Implement Phase 1 (one-way, human-move detection)
2. Run in production; gather feedback on conflicts and human moves
3. Implement Phase 2 (bidirectional) based on real usage patterns
4. Later: add explicit import mapping if needed

This approach validates the model before committing to complex bidirectional semantics.
