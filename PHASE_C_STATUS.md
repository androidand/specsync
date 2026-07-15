# Phase C: Test Coverage — Status & Implementation Guide

**Status**: PLANNING PHASE COMPLETE  
**Deliverable**: Comprehensive test coverage plan (20-38 new tests across 5 areas)  
**Target**: 160+ tests passing (up from 139 current)  
**Effort Estimate**: 1-2 days to full implementation  

---

## What Has Been Delivered (This Session)

### 1️⃣ TEST_COVERAGE_PLAN.md (402 lines)

A **comprehensive test specification** document outlining:

✅ **5 test areas identified**:
- Priority dispatch edge cases (5-8 tests)
- Board reconciliation scenarios (6-10 tests)
- State derivation precedence (4-6 tests)
- Error validation cases (5-6 tests)
- Integration workflows (5-8 tests)

✅ **Detailed test scenarios** for each area:
- Specific test function names and inputs
- Expected outputs and edge cases
- Why each test matters (safety-critical features highlighted)

✅ **Implementation priority** (Phase 1 vs Phase 2):
- Critical first (3-4 days): Board reconciliation, state derivation, error validation
- Important second (2-3 days): Priority dispatch, integration scenarios

✅ **Testing checklist**:
- Before adding tests (understand existing patterns)
- While writing tests (best practices)
- After writing tests (validation & coverage measurement)

✅ **Success criteria**:
- All tests passing: `go test -v ./...`
- Coverage > 80%: `go test -cover ./...`
- No data races: `go test -race ./...`
- Critical invariants verified (human-move detection, archived immutability, etc.)

---

## Test Coverage Requirements (From POLISH_SUMMARY)

**From Phase D documentation**, 5 specific areas need testing:

1. **Priority dispatch edge cases**
   - Null priority handling
   - Priority bounds (1-100)
   - Tie-breaking by creation date
   - Semantic tier definitions

2. **Board reconciliation scenarios**
   - Human moved card → skip update ✅ CRITICAL
   - Local changed, remote didn't → push update
   - Both changed → report conflict
   - First sync → create binding

3. **State derivation precedence**
   - Archived > metadata > legacy-status > task-derived > default
   - Missing metadata.json handling
   - Missing .specsync/ directory handling
   - Malformed metadata.json handling

4. **Error cases**
   - Invalid priority (out of range)
   - Invalid stage
   - Invalid slug (path traversal)
   - Change not found
   - Archived immutability

5. **Integration scenarios**
   - `specsync set-priority` → priority persists → queue reflects
   - Board moves → three-way merge respects → no clobbering
   - Changelog includes priority/stage changes
   - Old configs migrate gracefully

---

## Current Test Status

**Baseline**: 139 tests passing in v0.7.0  
**Coverage gaps**: No tests yet for priority/stage/board features  

**Where existing tests cover v0.7.0 features**:
- `specsync_test.go` — basic CLI operations
- `board_test.go` — board projection logic
- `pull_test.go` — GitHub issue → change conversion
- `change_test.go` — Change struct validation

**What's missing**:
- Priority sorting and validation (0 tests)
- Three-way merge logic (0 tests)
- State derivation precedence (0 tests)
- Archived change immutability (0 tests)
- Error message validation (minimal)

---

## Implementation Path (Recommended Order)

### Phase 1: Critical (3-4 days)

**Priority**: Board reconciliation (SAFETY-CRITICAL)

```bash
# Extend board_test.go with tests:
✅ TestThreeWayMergeNoChange
✅ TestThreeWayMergeLocalChanged
✅ TestThreeWayMergeRemoteChanged (human move detected)
✅ TestThreeWayMergeConflict
✅ TestThreeWayMergeHumanMoveDetection
✅ TestThreeWayMergeHumanMoveRespected (prevents sync)
```

**Priority**: State derivation (CORRECTNESS-CRITICAL)

```bash
# Create state_derivation_test.go:
✅ TestStagePrecedenceArchived
✅ TestStagePrecedenceMetadata
✅ TestStagePrecedenceLegacy
✅ TestStagePrecedenceDerived
✅ TestStagePrecedenceDefault
✅ TestStagePrecedenceImmutable
```

**Priority**: Error validation (USER-FACING)

```bash
# Add to test files:
✅ TestValidateStageBounds
✅ TestSetStageOnArchivedRejects
✅ TestPriorityOutOfRange
✅ TestSlugPathTraversalRejection
✅ TestBoardTargetFormatValidation
✅ TestMetadataJSONCorruptionHandling
```

**Success metric**: Human-move detection thoroughly tested; no regressions in board sync.

### Phase 2: Important (2-3 days)

**Priority**: Priority dispatch (CORE FEATURE)

```bash
# Create priority_dispatch_test.go:
✅ TestPriorityNilHandling
✅ TestPrioritySortingDescending
✅ TestPriorityTiebreakerByCreationDate
✅ TestPriorityValidationBounds
✅ TestPrioritySemanticTiers
```

**Priority**: Integration workflows (FULL FLOW)

```bash
# Create integration_workflow_test.go:
✅ TestEndToEndSetPriorityAffectsQueue
✅ TestEndToEndBoardMoveDetection
✅ TestEndToEndArchiveImmutability
✅ TestEndToEndMetadataAccuracy
✅ TestEndToEndConflictReporting
✅ TestEndToEndNilPriorityMigration
✅ TestEndToEndMultiBoardSync
```

**Success metric**: End-to-end workflows tested; priority sorting verified.

---

## Critical Tests to Prioritize

**MUST TEST** (safety-critical):
1. Three-way merge human-move detection → prevents sync clobbering
2. Archived change immutability → prevents accidental mutation
3. Priority sorting with nil values → correct queue order
4. State derivation precedence → correct stage calculation

**SHOULD TEST** (user-facing):
5. Error messages are clear and actionable
6. Validation catches invalid input early
7. Integration workflows succeed end-to-end

**NICE TO TEST** (optimization):
8. Performance benchmarks for 1000+ changes
9. Concurrent board sync safety
10. Metadata round-trip accuracy

---

## File Organization

```
cmd/specsync/
├── specsync_test.go          (existing: 139 tests)
├── board_test.go             (existing: extend with 3-way merge)
├── state_derivation_test.go   (new: precedence logic)
├── priority_dispatch_test.go  (new: priority sorting)
├── error_validation_test.go   (new: validation errors)
└── integration_workflow_test.go (new: end-to-end)
```

**Total lines**: ~1,500-2,000 lines of test code  
**Total test functions**: 25-38 new tests  
**Estimated completion time**: 1-2 working days  

---

## Known Challenges & Solutions

### Challenge 1: Testing three-way merge without external board

**Solution**: Use fakeBoard pattern (from existing board_test.go)

```go
type fakeBoard struct {
	localStage   Stage
	remoteStage  Stage
	baseLocal    Stage
	baseRemote   Stage
}

func (fb *fakeBoard) testThreeWayMerge(t *testing.T) {
	// Inject fake state and verify merge logic
}
```

### Challenge 2: Testing priority sorting with nil pointers

**Solution**: Create helper to construct test fixtures

```go
func ptrInt(v int) *int { return &v }

func mkChange(slug string, p *int, d time.Time) *Change {
	return &Change{Slug: slug, Priority: p, CreatedAt: d}
}

// Usage:
mkChange("a", nil, time.Now().Add(-1*time.Hour))
mkChange("b", ptrInt(50), time.Now())
```

### Challenge 3: Testing state derivation precedence with partial data

**Solution**: Table-driven tests with clear expected values

```go
type stateTest struct {
	name       string
	archived   bool
	meta       *ChangeMetadata
	legacy     string
	tasksAll   bool
	expected   Stage
	source     StageSource
}

var tests = []stateTest{
	{"archived wins all", true, &ChangeMetadata{Stage: "active"}, "done", true, StageArchived, StageSourceArchived},
	{"metadata wins legacy", false, &ChangeMetadata{Stage: "in-review"}, "done", true, StageInReview, StageSourceMetadata},
	// ...
}
```

### Challenge 4: Testing error messages consistency

**Solution**: Extract error message patterns and test each

```go
func TestErrorMessageConsistency(t *testing.T) {
	tests := []struct {
		input       string
		wantContains []string // "error:", "Expected format", example, etc.
	}{
		{"101", []string{"priority", "1", "100"}},
		{"../../etc", []string{"invalid slug", "path separator"}},
	}
}
```

---

## Testing Infrastructure (Already Available)

**Existing test helpers** in `specsync_test.go`:
- `tempDir()` — Creates isolated test directories
- `mkChange()` — Constructs Change fixtures
- `MockProvider` — Fakes external dependencies
- `fakeBoard` — Mocks board state

**Testing patterns already used**:
- Table-driven tests (most common)
- Subtests with `t.Run()`
- Fixture setup/teardown
- Assertion helpers

**No new test infrastructure needed** — reuse existing patterns.

---

## Validation Strategy

### Run tests incrementally

```bash
# After implementing Phase 1 (board + state + errors)
go test -v ./cmd/specsync -run "ThreeWayMerge|StagePrecedence|Validation"
go test -cover ./cmd/specsync  # Should be > 60%

# After implementing Phase 2 (priority + integration)
go test -v ./cmd/specsync
go test -cover ./cmd/specsync  # Should be > 80%
go test -race ./cmd/specsync   # Check for data races
```

### Coverage report

```bash
go test -cover ./cmd/specsync
# Expected output:
# coverage: 82.5% of statements

# Identify gaps:
go test -coverprofile=coverage.out ./cmd/specsync
go tool cover -html=coverage.out  # View in browser
```

### Manual testing checklist

- [ ] `specsync set-priority my-change 85` updates metadata.json correctly
- [ ] `specsync queue` sorts by priority (higher first)
- [ ] Human board move is detected and sync skipped
- [ ] Archived change rejects `set-stage` and `set-priority`
- [ ] Error messages are helpful and include recovery steps

---

## Success Criteria (Gate for Phase C Completion)

✅ **All new tests passing**:
```bash
go test -v ./cmd/specsync 2>&1 | grep -c "ok "  # Should be 160+
```

✅ **Coverage > 80%**:
```bash
go test -cover ./cmd/specsync | grep "coverage:"  # Should show > 80%
```

✅ **No data races**:
```bash
go test -race ./cmd/specsync  # Should complete without -race failures
```

✅ **Critical invariants verified**:
- Human-move detection: Has dedicated test ✅
- Archived immutability: Has dedicated test ✅
- Priority sorting: Has dedicated test ✅
- State derivation: Has 6+ tests ✅

✅ **Error scenarios covered**:
- Invalid priority: Has dedicated test ✅
- Invalid stage: Has dedicated test ✅
- Invalid slug: Has dedicated test ✅
- Out of range: Has dedicated test ✅

✅ **Integration workflows tested**:
- Set priority → queue reflects: Has dedicated test ✅
- Board move → preserved: Has dedicated test ✅
- Archive → immutable: Has dedicated test ✅
- Metadata round-trip: Has dedicated test ✅

---

## Next Steps to Execute

1. **Read existing test patterns** in `specsync_test.go` and `board_test.go`
2. **Create state_derivation_test.go** with 6 precedence tests
3. **Extend board_test.go** with 6-10 three-way merge tests
4. **Add error_validation_test.go** with 5-6 error tests
5. **Create priority_dispatch_test.go** with 5-8 priority tests
6. **Create integration_workflow_test.go** with 5-8 end-to-end tests
7. **Run full test suite**: `go test -v ./cmd/specsync`
8. **Measure coverage**: `go test -cover ./cmd/specsync`
9. **Check for races**: `go test -race ./cmd/specsync`
10. **Update documentation** with final test count and coverage metrics

---

## Questions for Implementation

These can be answered during Phase 1:

- Should benchmarks be added for priority sorting at 1000+ changes?
- Should stress tests be added (priority changing 100x rapidly)?
- Should integration tests use mocked GitHub provider or real API (with dry-run)?
- Should error message tests be automated or manual?
- Is performance regression testing needed for board sync?

---

## Phase C Deliverables Summary

| Deliverable | Status | Notes |
|-------------|--------|-------|
| Test plan document | ✅ Complete | TEST_COVERAGE_PLAN.md (402 lines) |
| Test scenarios specification | ✅ Complete | 25-38 tests specified across 5 areas |
| Implementation roadmap | ✅ Complete | Phase 1 (critical) and Phase 2 (important) |
| Known challenges & solutions | ✅ Complete | 4 key challenges addressed |
| Testing infrastructure | ✅ Available | Reuse existing patterns from specsync_test.go |
| Success criteria | ✅ Defined | Coverage > 80%, all critical invariants tested |

**Remaining work**: Execute test implementation (1-2 days)

---

## Conclusion

**Option C: Test Coverage** is now at the **specification & planning phase**.

The comprehensive test plan is ready to execute. All 5 test areas have:
- ✅ Specific test scenarios
- ✅ Expected inputs/outputs
- ✅ Critical invariants to verify
- ✅ Implementation order (Phase 1 critical first)

**Ready to proceed with implementation** when requested.

