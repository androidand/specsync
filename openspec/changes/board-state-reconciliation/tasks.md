# Tasks — Phase 1: One-Way Projection with Human-Move Detection

## 1. BoardBinding Schema & Persistence

- [ ] 1.1 Define BoardBinding struct: provider, projectId, itemId, base (localStage + remoteOptionId), syncedAt
- [ ] 1.2 Define MergeBase struct: localStage, remoteStatusOptionId
- [ ] 1.3 Define BoardState struct: version, bindings map[key]BoardBinding
- [ ] 1.4 Implement BoardState.Save(): write to .specsync/board.json atomically
- [ ] 1.5 Implement BoardState.Load(): read from .specsync/board.json; tolerate missing/malformed (disposable cache)
- [ ] 1.6 .specsync/board.json is gitignored

## 2. GraphQL Query Extensions

- [ ] 2.1 Extend resolveStatus() to also fetch option ID for comparison
- [ ] 2.2 Extend query to include updatedAt timestamp
- [ ] 2.3 Parse response to extract {optionId, name, updatedAt}
- [ ] 2.4 Tests: mock GraphQL responses with various option IDs

## 3. Three-Way Merge Algorithm (Phase 1)

- [ ] 3.1 Implement detectChanges(base MergeBase, currentLocal Stage, currentRemote OptionID) (localChanged bool, remoteChanged bool)
- [ ] 3.2 Implement mergeDecision() switch statement:
  - [ ] 3.2a No changes: no action
  - [ ] 3.2b Local only: push local stage to board
  - [ ] 3.2c Remote only: report human move, do NOT import (Phase 1)
  - [ ] 3.2d Both: check convergence, otherwise report conflict
- [ ] 3.3 Return structured decision (action: none|push|report, details)

## 4. Human Move Detection & Reporting

- [ ] 4.1 Detect when remote status changed from base but local didn't
- [ ] 4.2 Generate report: "change <slug>: human moved board status to <name>, local stage is <stage>"
- [ ] 4.3 Include option ID in report for debugging
- [ ] 4.4 Emit to stdout (visible to user, clear intent)
- [ ] 4.5 Do NOT import as stage change (Phase 1 boundary)

## 5. Binding Persistence in Sync

- [ ] 5.1 When projecting to board, persist binding with current base state
- [ ] 5.2 Update base.LocalStage and base.RemoteOptionId after successful projection
- [ ] 5.3 Update syncedAt to current time
- [ ] 5.4 Handle binding key: format as "provider:owner/number" (e.g., "github-projects:owner/5")

## 6. Status Mapping for New Stages

- [ ] 6.1 Extend boardTarget.statusNameFor() to handle backlog, blocked, in-review (new stages)
- [ ] 6.2 Existing default mappings: backlog→Todo(?), blocked→Blocked, active→In Progress, in-review→Review, complete→Done, archived→Done
- [ ] 6.3 If stage has no mapping, skip with diagnostic (do not silently use arbitrary first option)
- [ ] 6.4 Update -status-map examples in SKILL.md

## 7. Archived Item Behavior

- [ ] 7.1 Detect archived changes when syncing to board
- [ ] 7.2 If item exists on board, set status to "Done" (or mapped terminal status)
- [ ] 7.3 Log: "archived change <slug>: set to Done on board"
- [ ] 7.4 Do not remove item from board (not destructive)

## 8. Sync Output & Diagnostics

- [ ] 8.1 Update sync summary to include binding updates
- [ ] 8.2 Report human moves: "change <slug>: board status was moved by human; local stage unchanged"
- [ ] 8.3 Report conflicts: "change <slug>: local and remote both changed; manual review needed"
- [ ] 8.4 Report successful projections: "updated board status for <slug>"

## 9. Tests: Binding Persistence

- [ ] 9.1 Binding saved after successful projection
- [ ] 9.2 Multiple bindings per change (different projects) coexist
- [ ] 9.3 Binding updated on re-sync (syncedAt changes)
- [ ] 9.4 Malformed board.json is safely ignored (cache is disposable)

## 10. Tests: Three-Way Merge

- [ ] 10.1 No change: base == local && base == remote; skip
- [ ] 10.2 Local only: local != base && remote == base; push
- [ ] 10.3 Remote only: local == base && remote != base; report (don't import)
- [ ] 10.4 Both converged: local != base && remote != base && mapped(local) == remote; accept
- [ ] 10.5 Conflict: local != base && remote != base && mapped(local) != remote; report

## 11. Tests: Human Move Reporting

- [ ] 11.1 Human moves card to "Done"; local unchanged; report includes both states
- [ ] 11.2 Human moves card backward (regression); still reported
- [ ] 11.3 No spurious reports when base == remote

## 12. Tests: Archived Items

- [ ] 12.1 Archived change on board: status set to "Done"
- [ ] 12.2 Archived change not on board: no mutation

## 13. Documentation

- [ ] 13.1 Update SKILL.md: new status mappings for backlog, blocked, in-review
- [ ] 13.2 Document Phase 1 behavior: push only, preserve human moves, report conflicts
- [ ] 13.3 Note that bidirectional import is Phase 2 (future)
- [ ] 13.4 Add example sync output with human-move reporting
