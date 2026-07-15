# Implementation Tasks

## Phase 1: Interface & Structure (Day 1-2)

### Define SpecSource Interface
- [ ] Create `pkg/spec/spec.go`
- [ ] Define `SpecSource` interface with Name(), LoadChanges(), SaveChange()
- [ ] Document interface contract and error handling
- [ ] Test: interface is well-formed (compiles)

### Create OpenSpec Implementation
- [ ] Create `pkg/spec/openspec.go`
- [ ] Implement OpenSpecSource.Name() → "openspec"
- [ ] Move existing `pkg/openspec/load.go` logic into OpenSpecSource.LoadChanges()
- [ ] Handle all existing code paths (archive/, metadata.json, legacy .status, etc.)
- [ ] Test: OpenSpecSource loads all test fixtures correctly

### Create Beads Placeholder
- [ ] Create `pkg/spec/beads.go`
- [ ] Implement BeadsSource stub (returns "not implemented")
- [ ] Document Beads format requirements (for future implementation)
- [ ] Test: Beads source compiles and has expected interface

### Test Refactoring
- [ ] Move `pkg/openspec/openspec_test.go` to `pkg/spec/openspec_test.go`
- [ ] Refactor tests to use OpenSpecSource directly
- [ ] Add tests for interface compliance
- [ ] All existing tests pass with new structure

---

## Phase 2: Integration (Day 2-3)

### Update Options & Main Loop
- [ ] Add `SpecSource SpecSource` field to Options struct
- [ ] Set default: `if opts.SpecSource == nil { opts.SpecSource = OpenSpecSource{} }`
- [ ] Update all functions that call `openspec.LoadChanges()`:
  - [ ] `Sync()` in sync.go
  - [ ] `Pull()` in pull.go
  - [ ] `LoadChanges()` in commands/changes.go
- [ ] Use `opts.SpecSource.LoadChanges()` instead
- [ ] Test: all commands still work with default OpenSpec

### Update Existing Code
- [ ] `pkg/openspec/load.go` → thin wrapper (delegates to OpenSpecSource)
- [ ] Update imports (some packages move from openspec to spec)
- [ ] Verify backward compatibility: `openspec.LoadChanges()` still works
- [ ] All existing CLI commands unchanged

### CLI Integration
- [ ] Add `--spec` flag to main.go (optional, defaults to "openspec")
- [ ] Map flag value to SpecSource (factory function)
- [ ] Error handling: invalid spec source → fail with helpful message
- [ ] Test: `specsync sync --spec openspec` works
- [ ] Test: `specsync sync --spec beads` returns "not implemented"

---

## Phase 3: Testing & Validation (Day 3)

### Unit Tests
- [ ] TestOpenSpecSource_Name() → returns "openspec"
- [ ] TestOpenSpecSource_LoadChanges_SingleChange()
- [ ] TestOpenSpecSource_LoadChanges_Archive()
- [ ] TestOpenSpecSource_LoadChanges_WithMetadata()
- [ ] TestOpenSpecSource_LoadChanges_LegacyStatus()
- [ ] TestOpenSpecSource_LoadChanges_NoChanges() → empty slice, no error
- [ ] TestBeadsSource_NotImplemented()

### Integration Tests
- [ ] TestSync_WithOpenSpecSource_Default()
- [ ] TestSync_WithOpenSpecSource_Explicit()
- [ ] TestSync_WithBeadsSource_Error()
- [ ] TestPull_WithOpenSpecSource()
- [ ] TestChanges_ListWithOpenSpecSource()

### Backward Compatibility Tests
- [ ] Test: old `openspec.LoadChanges()` wrapper still works
- [ ] Test: all existing CLI workflows unchanged
- [ ] Test: performance regression check (should be zero)
- [ ] Test: error handling (malformed specs, missing dirs)

### Manual Testing
- [ ] `specsync changes` works (table output)
- [ ] `specsync changes --output json` works
- [ ] `specsync sync --dry-run` works
- [ ] `specsync pull` works
- [ ] `specsync link` works (uses SpecSource indirectly)

---

## Phase 4: Documentation (Day 4)

### Documentation Updates
- [ ] Update README.md: add "Pluggable Spec Sources" section
- [ ] Document how to add a new spec format (Beads example)
- [ ] Add code example: implementing custom SpecSource
- [ ] Document --spec CLI flag
- [ ] Update godoc comments for SpecSource interface

### Examples & Guides
- [ ] Create example: BeadsSource skeleton (for future implementation)
- [ ] Add troubleshooting: "How to use with Beads"
- [ ] Document future roadmap: OpenSpec (now), Beads (Phase 7+), others

### Configuration
- [ ] Add `spec_source: openspec` to config-specsync-example.yaml
- [ ] Document in WORKFLOW.md

---

## Phase 5: Cleanup & Polish (Day 4-5)

### Code Cleanup
- [ ] Remove or deprecate duplicate code paths
- [ ] Verify no dead code remains
- [ ] Check imports (fix circular dependencies if any)
- [ ] Run linters: golangci-lint, go fmt
- [ ] Verify no unused variables or functions

### Testing Completeness
- [ ] All tests passing: `go test ./...`
- [ ] Coverage should not drop from current baseline
- [ ] Add any missing edge case tests

### Final Validation
- [ ] Build: `go build ./cmd/specsync`
- [ ] Install: `npm install -g .` (if packaging)
- [ ] Full workflow test: plan → sync → pull → board
- [ ] Performance benchmark: no regression

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
