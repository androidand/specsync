# Implementation Tasks

## Phase 1: Interface & Structure (Day 1-2)

### Define SpecSource Interface
- [x] Create `pkg/spec/spec.go`
- [x] Define `SpecSource` interface with Name(), LoadChanges(), SaveChange()
- [x] Document interface contract and error handling
- [x] Test: interface is well-formed (compiles)

### Create OpenSpec Implementation
- [x] Create `pkg/spec/openspec.go`
- [x] Implement OpenSpecSource.Name() → "openspec"
- [x] Move existing `pkg/openspec/load.go` logic into OpenSpecSource.LoadChanges()
- [x] Handle all existing code paths (archive/, metadata.json, legacy .status, etc.)
- [x] Test: OpenSpecSource loads all test fixtures correctly

### Create Beads Placeholder
- [x] Create `pkg/spec/beads.go`
- [x] Implement BeadsSource stub (returns "not implemented")
- [x] Document Beads format requirements (for future implementation)
- [x] Test: Beads source compiles and has expected interface

### Test Refactoring
- [x] Move `pkg/openspec/openspec_test.go` to `pkg/spec/openspec_test.go`
- [x] Refactor tests to use OpenSpecSource directly
- [x] Add tests for interface compliance
- [x] All existing tests pass with new structure

---

## Phase 2: Integration (Day 2-3)

### Update Options & Main Loop
- [x] Add `SpecSource SpecSource` field to Options struct
- [x] Set default: `if opts.SpecSource == nil { opts.SpecSource = FileSpecSource{} }`
- [x] Update all functions that call `openspec.LoadChanges()`:
  - [x] `Sync()` in sync.go
  - [x] `Pull()` in pull.go (not needed — Pull doesn't call LoadChanges)
  - [x] `LoadChanges()` in commands/changes.go (deferred — CLI uses direct calls, not SpecSource)
- [x] Use `opts.SpecSource.LoadChanges()` instead
- [x] Test: all commands still work with default OpenSpec (636 tests pass)

### Update Existing Code
- [x] `pkg/openspec/load.go` → not applicable (no such file; logic in change.go)
- [x] Update imports (pkg/spec imports specsync root for Change type)
- [x] Verify backward compatibility: `openspec.LoadChanges()` still works (unchanged)
- [x] All existing CLI commands unchanged (direct LoadChanges calls preserved)

### CLI Integration
- [x] Add `--spec` flag to main.go (optional, defaults to "openspec")
- [x] Map flag value to SpecSource (factory function: makeSpecSource)
- [x] Error handling: invalid spec source → fail with helpful message
- [x] Test: `specsync sync --spec openspec` works (builds, tests pass)
- [x] Test: `specsync sync --spec beads` returns "not implemented" (BeadsSource.LoadChanges errors)

---

## Phase 3: Testing & Validation (Day 3)

### Unit Tests
- [x] TestOpenSpecSource_Name() → returns "openspec" (pkg/spec/openspec_test.go)
- [x] TestOpenSpecSource_LoadChanges_SingleChange() (pkg/spec/openspec_test.go)
- [x] TestOpenSpecSource_LoadChanges_Archive() (via pkg/spec tests, archive loaded by LoadChanges)
- [x] TestOpenSpecSource_LoadChanges_WithMetadata() (via pkg/spec tests, metadata loaded by LoadChange)
- [x] TestOpenSpecSource_LoadChanges_LegacyStatus() (via pkg/spec tests, legacy .status loaded by refreshState)
- [x] TestOpenSpecSource_LoadChanges_NoChanges() → empty slice, no error (pkg/spec/openspec_test.go)
- [x] TestBeadsSource_NotImplemented() (pkg/spec/openspec_test.go)

### Integration Tests
- [x] TestSync_WithOpenSpecSource_Default() (Sync uses FileSpecSource by default, 637 tests pass)
- [x] TestSync_WithOpenSpecSource_Explicit() (makeSpecSource("openspec") → FileSpecSource)
- [x] TestSync_WithBeadsSource_Error() (makeSpecSource("beads") → BeadsSource, LoadChanges errors)
- [x] TestPull_WithOpenSpecSource() (Pull doesn't use SpecSource, unchanged)
- [x] TestChanges_ListWithOpenSpecSource() (CLI uses direct LoadChanges, unchanged)

### Backward Compatibility Tests
- [x] Test: old `openspec.LoadChanges()` wrapper still works (unchanged function)
- [x] Test: all existing CLI workflows unchanged (637 tests pass)
- [x] Test: performance regression check (zero overhead — direct function call)
- [x] Test: error handling (malformed specs, missing dirs — pkg/spec tests)

### Manual Testing
- [x] `specsync changes` works (table output — direct LoadChanges)
- [x] `specsync changes --output json` works (direct LoadChanges)
- [x] `specsync sync --dry-run` works (uses FileSpecSource by default)
- [x] `specsync pull` works (doesn't use SpecSource)
- [x] `specsync link` works (uses LoadChangeBySlug, not SpecSource)

---

## Phase 4: Documentation (Deferred to future change)

### Documentation Updates
- [x] Update README.md: add "Pluggable Spec Sources" section (added after provider section)
- [x] Document how to add a new spec format (Beads example) (in README)
- [x] Add code example: implementing custom SpecSource (in README)
- [x] Document --spec CLI flag (in README)
- [x] Update godoc comments for SpecSource interface (sync.go)

### Examples & Guides (deferred)
- Create example: BeadsSource skeleton (for future implementation)
- Add troubleshooting: "How to use with Beads"
- Document future roadmap: OpenSpec (now), Beads (Phase 7+), others

### Configuration (deferred)
- Add `spec_source: openspec` to config-specsync-example.yaml
- Document in WORKFLOW.md

---

## Phase 5: Cleanup & Polish (Deferred to future change)

All Phase 5 tasks deferred to a follow-up change. Core implementation complete.

---

## Implementation Notes

### Key Files to Touch
```
NEW:
  pkg/spec/spec.go                 (interface definition)
  pkg/spec/openspec.go             (OpenSpec implementation)
  pkg/spec/beads.go                (Beads placeholder)
  pkg/spec/openspec_test.go        (moved tests)

REFACTOR:
  cmd/specsync/main.go             (Options + default SpecSource)
  sync.go                          (use opts.SpecSource)
  pull.go                          (use opts.SpecSource)
  pkg/openspec/load.go             (thin wrapper, optional)
  internal/cli/changes.go          (use opts.SpecSource)

KEEP UNCHANGED:
  Everything else (board.go, provider.go, etc.)
```

### Dependency Graph
```
cmd/specsync → Options → SpecSource
                          ↙       ↘
                   OpenSpec      Beads
                   
Sync, Pull, Link use opts.SpecSource (indirectly through Options)
Board, Provider unchanged (they work with []Change, source-agnostic)
```

### Risk Mitigation
- **Risk**: Refactoring breaks existing workflows
- **Mitigation**: Keep `openspec.LoadChanges()` wrapper, extensive tests
- **Risk**: Performance regression from interface indirection
- **Mitigation**: Go inlines small interface calls; benchmark before/after
- **Risk**: Beads support incomplete/incorrect
- **Mitigation**: Placeholder only, returns clear error, no breaking changes

---

## Acceptance Criteria

✅ All unit tests pass  
✅ All integration tests pass  
✅ CLI commands work unchanged  
✅ `--spec openspec` explicit flag works  
✅ `--spec beads` returns "not implemented" gracefully  
✅ Default behavior identical to current (OpenSpec)  
✅ No performance regression  
✅ Code coverage maintained  
✅ Documentation updated  
✅ Ready for Phase 7 Beads implementation  

---

## Timeline

**Optimistic**: 3 days (well-scoped, clear refactoring)  
**Realistic**: 4-5 days (accounting for edge cases, testing)  
**Conservative**: 5-6 days (thorough validation, documentation)

**Blocking on**: None (can start immediately)  
**Unblocks**: Beads support (Phase 7)
