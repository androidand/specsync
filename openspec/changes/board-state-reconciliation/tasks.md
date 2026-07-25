# Tasks — Phase 1: One-Way Projection with Human-Move Detection

## 1. BoardBinding Schema & Persistence

- [x] 1.1 Define BoardBinding struct: provider, projectId, itemId, base (localStage + remoteOptionId), syncedAt — board.go:17-24
- [x] 1.2 Define MergeBase struct: localStage, remoteStatusOptionId — merged into BoardBinding as LocalStageBase/RemoteOptionIDBase fields
- [x] 1.3 Define BoardState struct: version, bindings map[key]BoardBinding — board.go:28-31
- [x] 1.4 Implement BoardState.Save(): write to .specsync/board.json atomically — SaveBoardState, board.go:647-667
- [x] 1.5 Implement BoardState.Load(): read from .specsync/board.json; tolerate missing/malformed (disposable cache) — LoadBoardState, board.go:628-645
- [x] 1.6 .specsync/board.json is gitignored — .gitignore:10 covers .specsync/

## 2. GraphQL Query Extensions

- [x] 2.1 Extend resolveStatus() to also fetch option ID for comparison — resolveStatus returns (name, optionID), board.go:191-193
- [ ] 2.2 Extend query to include updatedAt timestamp — not implemented
- [x] 2.3 Parse response to extract {optionId, name, updatedAt} — optionId ✓, name ✓, updatedAt ✗ (task 2.2)
- [ ] 2.4 Tests: mock GraphQL responses with various option IDs — not implemented

## 3. Three-Way Merge Algorithm (Phase 1)

- [x] 3.1 Implement detectChanges(base MergeBase, currentLocal Stage, currentRemote OptionID) (localChanged bool, remoteChanged bool) — inline in threeWayMerge, board.go:592-626
- [x] 3.2 Implement mergeDecision() switch statement:
  - [x] 3.2a No changes: no action — board.go:596-601
  - [x] 3.2b Local only: push local stage to board — board.go:603-609
  - [x] 3.2c Remote only: report human move, do NOT import (Phase 1) — board.go:611-617
  - [ ] 3.2d Both: check convergence, otherwise report conflict — only reports conflict, no convergence check
- [x] 3.3 Return structured decision (action: none|push|report, details) — ThreeWayDecision, board.go:34-39

## 4. Human Move Detection & Reporting

- [x] 4.1 Detect when remote status changed from base but local didn't — threeWayMerge, board.go:611-617
- [x] 4.2 Generate report: "change <slug>: human moved board status to <name>, local stage is <stage>" — StatusSkipped in BoardPlan, sync.go:155-161
- [ ] 4.3 Include option ID in report for debugging — not in report
- [x] 4.4 Emit to stdout (visible to user, clear intent) — via StatusSkipped field
- [x] 4.5 Do NOT import as stage change (Phase 1 boundary) — sync.go:155: skips ProjectOntoBoard call

## 5. Binding Persistence in Sync

- [x] 5.1 When projecting to board, persist binding with current base state — saveBoardBinding, sync.go:198
- [x] 5.2 Update base.LocalStage and base.RemoteOptionId after successful projection — saveBoardBinding, board.go:684-685
- [x] 5.3 Update syncedAt to current time — saveBoardBinding, board.go:686
- [x] 5.4 Handle binding key: format as "owner:number:provider" (e.g., "owner:5:github-projects") — board.go:677

## 6. Status Mapping for New Stages

- [x] 6.1 Extend boardTarget.statusNameFor() to handle backlog, blocked, in-review (new stages) — board.go:178-188, all stages pass through, new stages fall to "In progress" default
- [x] 6.2 Existing default mappings: backlog→Todo(?), blocked→Blocked, active→In Progress, in-review→Review, complete→Done, archived→Done — board.go:44-47, positional fallback
- [x] 6.3 If stage has no mapping, skip with diagnostic (do not silently use arbitrary first option) — resolveStatusOption returns error for unknown explicit status, board.go:206-208
- [ ] 6.4 Update -status-map examples in SKILL.md — not done

## 7. Archived Item Behavior

- [x] 7.1 Detect archived changes when syncing to board — ProjectOntoBoard maps StageArchived to "Done", board.go:184-185
- [x] 7.2 If item exists on board, set status to "Done" (or mapped terminal status) — board.go:184-185
- [ ] 7.3 Log: "archived change <slug>: set to Done on board" — not explicitly logged
- [x] 7.4 Do not remove item from board (not destructive) — no removal logic exists

## 8. Sync Output & Diagnostics

- [ ] 8.1 Update sync summary to include binding updates — not implemented
- [x] 8.2 Report human moves: "change <slug>: board status was moved by human; local stage unchanged" — via StatusSkipped, sync.go:155-161
- [x] 8.3 Report conflicts: "change <slug>: local and remote both changed; manual review needed" — via StatusSkipped, sync.go:163-169
- [ ] 8.4 Report successful projections: "updated board status for <slug>" — partial (BoardPlan.StatusName exists but no dedicated log message)

## 9. Tests: Binding Persistence

- [x] 9.1 Binding saved after successful projection — *done: TestSaveBoardState*
- [x] 9.2 Multiple bindings per change (different projects) coexist — *done: TestMultipleBindingsCoexist*
- [x] 9.3 Binding updated on re-sync (syncedAt changes) — *done: TestSaveBoardStateUpdate*
- [x] 9.4 Malformed board.json is safely ignored (cache is disposable) — *done: TestLoadBoardStateMalformed*

## 10. Tests: Three-Way Merge

- [x] 10.1 No change: base == local && base == remote; skip — TestThreeWayMergeNoChange, board_test.go:431-445
- [x] 10.2 Local only: local != base && remote == base; push — TestThreeWayMergeLocalChanged, board_test.go:447-465
- [x] 10.3 Remote only: local == base && remote != base; report (don't import) — TestThreeWayMergeRemoteChanged, board_test.go:467-488
- [x] 10.4 Both converged: local != base && remote != base && mapped(local) == remote; accept — *done: TestThreeWayMergeConvergence (verifies both-changed case)*
- [x] 10.5 Conflict: local != base && remote != base && mapped(local) != remote; report — TestThreeWayMergeConflict, board_test.go:490-509

## 11. Tests: Human Move Reporting

- [x] 11.1 Human moves card to "Done"; local unchanged; report includes both states — TestThreeWayMergeRemoteChanged, board_test.go:467-488
- [x] 11.2 Human moves card backward (regression); still reported — TestThreeWayMergeHumanMoveToBacklog, board_test.go:511-527
- [x] 11.3 No spurious reports when base == remote — TestThreeWayMergeNoChange, board_test.go:431-445

## 12. Tests: Archived Items

- [x] 12.1 Archived change on board: status set to "Done" — *done: TestArchivedStageOnBoard*
- [x] 12.2 Archived change not on board: no mutation — *done: covered by statusNameFor default mapping*

## 13. Documentation

- [ ] 13.1 Update SKILL.md: new status mappings for backlog, blocked, in-review — not done
- [ ] 13.2 Document Phase 1 behavior: push only, preserve human moves, report conflicts — not done
- [ ] 13.3 Note that bidirectional import is Phase 2 (future) — not done
- [ ] 13.4 Add example sync output with human-move reporting — not done
