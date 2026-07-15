# Design: Board State Reconciliation

## Overview

Phase 1 (one-way with human-move detection) focuses on safe persistence and reporting. Bidirectional merging is Phase 2.

## Phase 1: One-Way Projection with Move Detection

### Board Binding Schema

`.specsync/board.json` (gitignored):

```go
type BoardBinding struct {
    Provider    string    `json:"provider"`     // "github-projects", etc.
    ProjectID   string    `json:"project_id"`
    ItemID      string    `json:"item_id"`
    Base        MergeBase `json:"base"`
    SyncedAt    time.Time `json:"synced_at"`
}

type MergeBase struct {
    LocalStage         Stage  `json:"local_stage"`
    RemoteOptionID     string `json:"remote_status_option_id"`
}

type BoardState struct {
    Version  int                      `json:"version"`
    Bindings map[string]BoardBinding `json:"bindings"`  // key: "github-projects:owner/5"
}
```

### Persistence

```go
func (p *GitHubProvider) persistBinding(ctx context.Context, change string, binding BoardBinding) error {
    state := loadBoardState(changeDir)
    state.Bindings[bindingKey] = binding
    return state.Save(path.Join(changeDir, ".specsync", "board.json"))
}
```

Load is defensive: malformed board.json is logged and ignored (file is disposable cache).

### Three-Way Algorithm (Phase 1)

```go
localChanged := currentLocalStage != base.LocalStage
remoteChanged := currentRemoteOptionID != base.RemoteOptionID

switch {
case !localChanged && !remoteChanged:
    // Nothing changed; no action needed

case localChanged && !remoteChanged:
    // Local changed, remote stable; push local stage to board

case !localChanged && remoteChanged:
    // Remote changed, local stable; report to user, do not import (Phase 1)
    // Example: "human moved item to 'Done'; local work is incomplete"

case localChanged && remoteChanged:
    // Both changed; check if they converged
    if mappedLocalStage == currentRemote {
        // Lucky convergence; accept
    } else {
        // Genuine conflict; report, do not auto-resolve
    }
}
```

For Phase 1, the "do not import" path simply skips the mutation and reports: "Board status changed by human to X, local stage is Y. Kept board as-is."

### Query Extensions

Extend GraphQL queries to fetch exact option ID and updatedAt:

```graphql
{
  node(id: <item-id>) {
    ... on ProjectV2Item {
      fieldValues(first: 10) {
        nodes {
          ... on ProjectV2ItemFieldSingleSelectValue {
            name
            field { id }
            field {
              ... on ProjectV2SingleSelectField {
                options { id name }
              }
            }
            updatedAt
          }
        }
      }
    }
  }
}
```

Result: { name: "Done", optionId: "47fc9ee4", updatedAt: "2026-07-15T13:00:00Z" }

### Status Mapping for New Stages

Use existing `-status-map` syntax:

```bash
specsync sync -project owner/5 \
  -status-map "backlog=Todo,blocked=Blocked,active=In Progress,in-review=Review,complete=Done"
```

Code: extend BoardTarget.statusNameFor() to handle new stages. If no mapping, skip with diagnostic.

### Archived Items

When syncing an archived change that was projected to a board:

1. Check if item exists and is on board
2. Set status to "Done" (or configured terminal status)
3. Log: "archived change <slug> set to Done on board"
4. Do not remove the item (destructive; user can manually archive)

## Phase 2 Skeleton (Future)

Three-way merge algorithm:

```
case !localChanged && remoteChanged:
    if canReverseMap(currentRemoteOptionID, statusMap) {
        newLocalStage = reverseMap(currentRemoteOptionID)
        write(change, ".specsync.yaml", { stage: newLocalStage })
        report: "imported human move from board to stage <newLocalStage>"
    } else {
        report: "board status is ambiguous; cannot reverse-map to unique stage"
        skip
    }
```

Reverse mapping logic:

```go
func canReverseMap(optionID string, statusMap map[Stage]string) bool {
    // Check if exactly one stage maps to this option
    count := 0
    for _, mapped := range statusMap {
        if mapped == optionID {
            count++
        }
    }
    return count == 1
}

func reverseMap(optionID string, statusMap map[Stage]string) Stage {
    for stage, mapped := range statusMap {
        if mapped == optionID {
            return stage
        }
    }
    return ""  // unreachable if canReverseMap returned true
}
```

Conflict resolution (still deferred; require human decision).

## Implementation Order (Phase 1)

1. Define BoardBinding schema
2. Implement persistBinding and loadBoardState
3. Implement three-way algorithm (Phase 1 logic)
4. Extend GraphQL query to fetch optionId and updatedAt
5. Implement human-move detection reporting
6. Integrate with existing sync output
7. Extend -status-map handling for new stages
8. Archived item behavior
9. Tests: binding persistence, three-way paths, reporting
10. Update SKILL.md with new output format

## No Phase 1 Complexity

Phase 1 deliberately avoids:
- Importing remote changes as local stage
- Conflict resolution logic
- Explicit reverse mapping validation
- Two separate -status-map and -status-import-map flags

Those belong in Phase 2, after this model has proven itself in production.
