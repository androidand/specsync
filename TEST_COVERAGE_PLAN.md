# Test Coverage Plan - Option C

## Overview

This document outlines comprehensive test coverage for priority dispatch, workflow state management, and board reconciliation features added in v0.7.0.

**Current Status**: 139 tests passing  
**Target**: 160+ tests (+20 new tests across 5 areas)  
**Effort**: 1-2 days for comprehensive coverage

---

## Test Areas & Implementation Plan

### 1️⃣ Priority Dispatch Edge Cases (5-8 tests needed)

**Location**: Add to existing `specsync_test.go` or create `priority_dispatch_test.go`

**Test scenarios to cover**:

```go
func TestPriorityNilHandling(t *testing.T) {
	// Test: nil priority defaults to 0 in sorting
	// Input: Change{Priority: nil} vs Change{Priority: ptr(50)}
	// Expected: Non-nil priority wins (sorts first if higher)
}

func TestPrioritySortingDescending(t *testing.T) {
	// Test: Higher priority sorts first
	// Input: [p=30, p=50, p=80, p=nil]
	// Expected: [p=80, p=50, p=30, p=nil]
}

func TestPriorityTiebreakerByCreationDate(t *testing.T) {
	// Test: Same priority uses creation date (older first)
	// Input: [newer(p=50), older(p=50)]
	// Expected: [older(p=50), newer(p=50)]
}

func TestPriorityValidationBounds(t *testing.T) {
	// Test: Priority must be 1-100 or nil
	// Invalid: 0, -1, 101, 999, "abc"
	// Valid: 1, 50, 99, 100, nil
}

func TestPrioritySemanticTiers(t *testing.T) {
	// Test: Verify tier definitions don't have gaps
	// FOCUS: 99
	// CRITICAL: 90-98
	// HIGH: 70-89
	// NORMAL: 50-69
	// LOW: 30-49
	// VERY_LOW: 1-29
	// DEFAULT: nil/0
}
```

**Why this matters**: Priority is the core feature driving work dispatch. Edge cases around nil values and sorting must be bulletproof.

---

### 2️⃣ Board Reconciliation - Three-Way Merge (6-10 tests needed)

**Location**: Extend `board_test.go` with 3-way merge scenarios

**Test scenarios to cover**:

```go
func TestThreeWayMergeNoChange(t *testing.T) {
	// Test: Both local and remote unchanged
	// Input: local=active, remote=in-progress, base.local=active, base.remote=in-progress
	// Expected: Action="none", skip=false
}

func TestThreeWayMergeLocalChanged(t *testing.T) {
	// Test: Local progressed, remote didn't
	// Input: local=complete, remote=in-progress, base.local=active, base.remote=in-progress
	// Expected: Action="push-local", skip=false
}

func TestThreeWayMergeRemoteChanged(t *testing.T) {
	// Test: Human moved card on board, local didn't change
	// Input: local=active, remote=done, base.local=active, base.remote=in-progress
	// Expected: Action="report-remote-move", skip=true (CRITICAL: prevents clobbering)
}

func TestThreeWayMergeConflict(t *testing.T) {
	// Test: Both local AND remote changed
	// Input: local=complete, remote=blocked, base.local=active, base.remote=in-progress
	// Expected: Action="report-conflict", skip=true
}

func TestThreeWayMergeHumanMoveToBacklog(t *testing.T) {
	// Test: Specific real scenario - human moved work back
	// Input: local=active, remote=backlog, base.local=active, base.remote=in-progress
	// Expected: Action="report-remote-move", reason contains "human"
}

func TestThreeWayMergeHumanMoveRespected(t *testing.T) {
	// Test: Verify human move prevents sync
	// Setup: Call threeWayMerge(...) for human move
	// Expected: Caller skips ProjectOntoBoard call (doesn't update board)
}
```

**Why this matters**: The three-way merge is what prevents accidentally clobbering human board moves. This is a safety-critical feature.

**Critical invariant to test**: "Human moved card on board" → "sync skipped" (no board update)

---

### 3️⃣ State Derivation Precedence (4-6 tests needed)

**Location**: Add to `change.go` test or create `state_derivation_test.go`

**Test scenarios to cover**:

```go
func TestStagePrecedenceArchived(t *testing.T) {
	// Test: archived always takes precedence
	// Input: Change{Archived: true, metadata.Stage: "active", ...}
	// Expected: c.Stage == StageArchived
}

func TestStagePrecedenceMetadata(t *testing.T) {
	// Test: metadata overrides legacy and derived
	// Input: metadata.Stage="in-review", legacyStatus="done", tasksComplete=true
	// Expected: c.Stage == StageInReview
}

func TestStagePrecedenceLegacy(t *testing.T) {
	// Test: legacy status used when no metadata
	// Input: metadata=nil, legacyStatus="done", ...
	// Expected: c.Stage == StageComplete
}

func TestStagePrecedenceDerived(t *testing.T) {
	// Test: tasks completion used when no metadata or legacy
	// Input: metadata=nil, legacy=nil, allTasksComplete=true
	// Expected: c.Stage == StageComplete
}

func TestStagePrecedenceDefault(t *testing.T) {
	// Test: backlog is default when nothing else set
	// Input: metadata=nil, legacy=nil, allTasksComplete=false
	// Expected: c.Stage == StageBacklog
}

func TestStagePrecedenceImmutable(t *testing.T) {
	// Test: archived can't be un-archived via metadata
	// Input: Change{Archived: true, metadata.Stage: "backlog"}
	// Expected: c.Stage == StageArchived (metadata ignored)
}
```

**Why this matters**: Stage derivation has 5 levels of precedence. All must be tested to verify the hierarchy works correctly and archived changes stay immutable.

---

### 4️⃣ Error Cases & Validation (5-6 tests needed)

**Location**: Add to relevant test files

**Test scenarios to cover**:

```go
func TestValidateStageBounds(t *testing.T) {
	// Test: Canonical stages pass, invalid stages fail
	// Valid: backlog, active, blocked, in-review, complete, archived
	// Valid custom: "custom-stage", "awaiting-review"
	// Invalid: "", "-invalid", "Has Space", "UPPERCASE", special chars
}

func TestSetStageOnArchivedRejects(t *testing.T) {
	// Test: Can't mutate archived change's stage
	// Input: archived change, set-stage command
	// Expected: error "cannot mutate archived change"
}

func TestPriorityOutOfRange(t *testing.T) {
	// Test: Priority must be 1-100 or unset
	// Input: 0, -1, 101, 999
	// Expected: error "priority must be between 1 and 100"
}

func TestSlugPathTraversalRejection(t *testing.T) {
	// Test: Can't use ".." or "/" in slug
	// Input: "../../etc/passwd", "foo/bar", "dir/../escape"
	// Expected: error "invalid slug"
}

func TestBoardTargetFormatValidation(t *testing.T) {
	// Test: Board target must be "owner/number" format
	// Input: "invalid", "owner", "owner/not-a-number"
	// Expected: error with expected format
}

func TestMetadataJSONCorruptionHandling(t *testing.T) {
	// Test: Malformed metadata.json doesn't crash
	// Input: {invalid json}, empty file, null bytes
	// Expected: error "invalid JSON" or defaults to empty metadata
}
```

**Why this matters**: Users encounter these edge cases. Comprehensive error messages matter.

---

### 5️⃣ Integration Scenarios (5-8 tests needed)

**Location**: Create `integration_workflow_test.go`

**Test scenarios to cover**:

```go
func TestEndToEndSetPriorityAffectsQueue(t *testing.T) {
	// Test: specsync set-priority changes queue order
	// 1. Create 3 changes: p=nil, p=30, p=80
	// 2. LoadChanges()
	// 3. Call set-priority on p=nil to p=85
	// 4. LoadChanges() again
	// Expected: p=85 now sorts first
}

func TestEndToEndBoardMoveDetection(t *testing.T) {
	// Test: Human board move preserved through full sync cycle
	// 1. Create binding: local=active, remote=in-progress
	// 2. Simulate human move: remote → done
	// 3. Run Sync()
	// Expected: Board NOT updated (human move preserved)
}

func TestEndToEndArchiveImmutability(t *testing.T) {
	// Test: Can't change archived change's priority or stage
	// 1. Archive a change
	// 2. Try: set-priority, set-stage
	// Expected: Both rejected with "cannot mutate archived"
}

func TestEndToEndMetadataAccuracy(t *testing.T) {
	// Test: metadata.json round-trips correctly
	// 1. Write metadata with priority=85, stage=in-review
	// 2. LoadChanges()
	// 3. Verify Change struct has correct priority and stage
	// 4. Write again
	// Expected: No data loss, format stable
}

func TestEndToEndConflictReporting(t *testing.T) {
	// Test: Conflicts reported to user, not auto-resolved
	// 1. Create scenario: local=complete, remote=blocked
	// 2. Run Sync()
	// 3. Check plan.StatusSkipped contains "conflict"
	// Expected: Board NOT updated, conflict logged
}

func TestEndToEndNilPriorityMigration(t *testing.T) {
	// Test: Old changes without metadata work smoothly
	// 1. Create change without .specsync/metadata.json
	// 2. LoadChanges()
	// 3. Verify: Priority=nil, Stage=backlog (default)
	// 4. Set priority to 50
	// Expected: No errors, metadata created
}

func TestEndToEndMultiBoardSync(t *testing.T) {
	// Test: Change can sync to multiple boards simultaneously
	// 1. Configure board targets for GitHub AND Jira
	// 2. Run Sync()
	// 3. Check: both board.json bindings updated
	// Expected: Multi-provider state tracked correctly
}
```

**Why this matters**: Integration tests verify the full flow works. These catch subtle interactions that unit tests miss.

---

## Implementation Priority

### Phase 1 (Critical) - 3-4 days
1. **Board reconciliation** (6-10 tests) — Prevents data loss
2. **State derivation** (4-6 tests) — Ensures correct stage calculation
3. **Error validation** (5-6 tests) — Catches user mistakes early

**Success metric**: Three-way merge thoroughly tested; human-move detection verified

### Phase 2 (Important) - 2-3 days
4. **Priority dispatch** (5-8 tests) — Core feature
5. **Integration scenarios** (5-8 tests) — Full workflow coverage

**Success metric**: End-to-end workflows tested; priority sorting verified

---

## Testing Checklist

### Before adding tests
- [ ] Read existing board_test.go to understand fakeBoard pattern
- [ ] Check specsync_test.go for test structure and helpers
- [ ] Verify all types/functions are exported (BoardBinding, threeWayMerge, etc.)
- [ ] Understand test data fixtures (temporary directories, changes)

### While writing tests
- [ ] One scenario per test function (clear failure messages)
- [ ] Use table-driven tests for related scenarios
- [ ] Create fixture helpers (mkChange, mkBoardBinding)
- [ ] Test both positive (should work) and negative (should reject)
- [ ] Comment WHY each test matters (not just WHAT it tests)

### After writing tests
- [ ] Run `go test -v ./... ` to verify all pass
- [ ] Run `go test -cover ./...` to check coverage
- [ ] Run `go test -race ./...` to catch concurrency issues
- [ ] Run integration tests against a fake board provider

---

## Coverage Goals

| Area | Current | Target | Tests to Add |
|------|---------|--------|--------------|
| Priority dispatch | ~10 | 18-25 | 5-8 |
| Board reconciliation | ~15 | 25-30 | 6-10 |
| State derivation | ~8 | 12-18 | 4-6 |
| Error validation | ~5 | 10-15 | 5-6 |
| Integration | ~0 | 8-15 | 5-8 |
| **Totals** | **38** | **73-103** | **25-38** |

**Note**: These are estimates. Actual count depends on how granular tests become.

---

## Test Organization

```
specsync_test.go          (existing: basics, sync, pull, link)
board_test.go             (existing: projection, will extend with 3-way merge)
stage_derivation_test.go   (new: precedence logic)
priority_dispatch_test.go  (new: priority sorting, edge cases)
error_validation_test.go   (new: validation error cases)
integration_workflow_test.go (new: end-to-end scenarios)
```

---

## Success Criteria

✅ **All tests passing**:
```bash
go test -v ./...
go test -cover ./...  # > 80% coverage
go test -race ./...   # No data races
```

✅ **Critical invariants verified**:
- [ ] Human board moves never clobbered
- [ ] Archived changes immutable
- [ ] Priority sorting correct (higher first, nil last)
- [ ] State derivation respects 5-level precedence

✅ **Error handling**:
- [ ] All validation errors have helpful messages
- [ ] No panics on edge cases
- [ ] Conflicts reported, not silently resolved

✅ **Integration**:
- [ ] Full workflow (set-priority → queue → sync) tested
- [ ] Multi-board scenarios tested
- [ ] Metadata round-trips correctly

---

## Notes

- Tests should run fast: < 1s per test
- Use `-parallel` flag for speed: `go test -parallel 8 ./...`
- Keep test fixtures simple (don't over-engineer setup)
- Test behaviors, not implementation details
- Document WHY each test matters (safety-critical features)

---

## Next Steps

1. **Review this plan** with the team
2. **Implement Phase 1** (board reconciliation + state derivation)
3. **Run coverage report** to find gaps
4. **Implement Phase 2** (priority + integration)
5. **Achieve 80%+ coverage** on critical paths

---

## Questions for Implementation

- Should we add benchmark tests for priority sorting at 1000+ changes?
- Should we test board sync performance under concurrent operations?
- Should we add stress tests (priority changing 100x rapidly)?
- Should integration tests mock external providers or use in-memory fixtures?

These questions can be answered during Phase 1 implementation.
