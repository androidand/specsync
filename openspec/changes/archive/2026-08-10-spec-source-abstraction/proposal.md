# Pluggable Spec Sources: Foundation for OpenSpec, Beads, and Beyond

**Goal**: Enable specsync to support multiple spec formats (OpenSpec, Beads, others) with zero runtime overhead and minimal code duplication.

## Problem

Currently, specsync is hardcoded for OpenSpec:
- Change loading assumes `openspec/changes/` directory structure
- Proposal and tasks file names are hardcoded (`proposal.md`, `tasks.md`)
- No abstraction between "what a spec is" and "how to load it"
- Future support for Beads or other formats would require monolithic refactoring

Result: We claim architectural flexibility ("pluggable providers") but the spec format is tightly coupled.

## Solution Overview

Introduce a `SpecSource` interface parallel to the existing `WorkProvider` interface.

This allows:
1. **Multiple spec formats**: OpenSpec (default), Beads, custom formats
2. **Zero coupling**: Main loop uses interface, not hardcoded OpenSpec
3. **Incremental adoption**: Start with OpenSpec, add Beads later
4. **Clean architecture**: Spec loading and issue tracking are independently pluggable

## Detailed Specification

### 1. SpecSource Interface

```go
// pkg/spec/spec.go
type SpecSource interface {
  // Name returns the spec source identifier (e.g., "openspec", "beads")
  Name() string
  
  // LoadChanges loads all changes from the spec directory.
  // Returns empty slice if no changes found (not an error).
  LoadChanges(specDir string) ([]Change, error)
  
  // SaveChange persists a change to disk (for future: metadata updates, etc.)
  SaveChange(change Change) error
}
```

### 2. OpenSpec Implementation

```go
// pkg/spec/openspec.go
type OpenSpecSource struct{}

func (s OpenSpecSource) Name() string {
  return "openspec"
}

func (s OpenSpecSource) LoadChanges(specDir string) ([]Change, error) {
  // Move existing openspec.LoadChanges() logic here
  // Returns []Change with:
  //   - Dir: openspec/changes/<slug>/
  //   - Title, Body, Tasks from proposal.md / tasks.md
  //   - Metadata from .specsync/metadata.json
}

func (s OpenSpecSource) SaveChange(change Change) error {
  // Future: write back to .specsync/metadata.json, etc.
  return nil
}
```

### 3. Beads Implementation (Placeholder)

```go
// pkg/spec/beads.go
type BeadsSource struct{}

func (s BeadsSource) Name() string {
  return "beads"
}

func (s BeadsSource) LoadChanges(specDir string) ([]Change, error) {
  // Load from Beads task graph format
  // TBD: understand Beads structure
  return nil, fmt.Errorf("beads support not yet implemented")
}

func (s BeadsSource) SaveChange(change Change) error {
  return fmt.Errorf("beads support not yet implemented")
}
```

### 4. Options & Main Loop

```go
type Options struct {
  SpecDir    string      // path to spec root (openspec/, beads/, etc.)
  SpecSource SpecSource  // injected, defaults to OpenSpecSource
  // ... rest of options
}

// Main loop (in sync.go, pull.go, etc.)
func Sync(ctx context.Context, opts Options) error {
  if opts.SpecSource == nil {
    opts.SpecSource = OpenSpecSource{}  // default
  }
  
  changes, err := opts.SpecSource.LoadChanges(opts.SpecDir)
  if err != nil {
    return err
  }
  
  // Rest of logic unchanged
}
```

### 5. CLI Behavior

Default: OpenSpec (no flag needed)
```bash
specsync sync
# Uses OpenSpec by default
```

Future: Support alternative specs via flag
```bash
specsync sync --spec beads
# Uses BeadsSource (when implemented)
```

### 6. Testing

- Unit tests for OpenSpecSource (existing, move unchanged)
- Integration tests with both OpenSpec and Beads (when Beads is ready)
- Mock SpecSource for dispatcher/board tests (isolate from file I/O)

## Implementation Notes

### Zero Cost Abstraction
- Interface method calls compile to direct function calls (no vtable overhead in Go)
- Default OpenSpec path unchanged (same performance)
- No behavioral changes, only structural

### Backward Compatibility
- Existing CLI, APIs, config all unchanged
- OpenSpec remains default and recommended format
- No migration required

### Future-Proofing
- Adding Beads support requires only:
  1. Implement BeadsSource interface
  2. Add CLI flag for `--spec beads`
  3. No changes to core logic (Sync, Pull, Board, etc.)
- Same story for other spec formats (GitLab Wiki, ADRs, etc.)

## Files to Create/Modify

**New**:
- `pkg/spec/spec.go` — SpecSource interface
- `pkg/spec/openspec.go` — Move OpenSpec loading logic here
- `pkg/spec/beads.go` — Placeholder for Beads (not implemented yet)
- `pkg/spec/openspec_test.go` — Move/refactor existing tests

**Refactor**:
- `pkg/openspec/load.go` — Becomes thin wrapper around OpenSpecSource
- `cmd/specsync/main.go` — Inject SpecSource into Options
- `sync.go`, `pull.go`, `link.go` — Use opts.SpecSource instead of direct calls

**Config**:
- `.skein/config.yaml` — Add `spec_source: openspec` (default, shown in example)

## Success Criteria

- ✅ SpecSource interface defined and documented
- ✅ OpenSpec implementation complete (all existing tests pass)
- ✅ Main loop uses interface, not hardcoded OpenSpec
- ✅ Beads placeholder exists (returns "not implemented")
- ✅ Zero behavioral changes (all existing workflows work identically)
- ✅ Zero performance regression
- ✅ CLI tests pass with both OpenSpec and Beads paths
- ✅ Documentation updated with "pluggable spec sources" section

## Open Questions

1. **Beads structure**: What does a Beads task look like? How to map to Change?
2. **Custom spec formats**: Should we document how to add a third format?
3. **CLI UX**: Should `--spec openspec` be required, or default silently?
4. **Error handling**: If spec format is unrecognized, fail early or report gracefully?

## Future Extensions

- **Beads support** (Phase 7+): Implement BeadsSource, wire into CLI
- **GitLab Wiki** (Phase 7+): Load specs from .wiki/ directory
- **ADRs** (Phase 7+): Load from adr/ (Architecture Decision Records)
- **Custom loader**: User-defined SpecSource for proprietary formats
- **Spec migration**: Tool to convert OpenSpec → Beads (or vice versa)

## Effort Estimate

- Refactoring: 2-3 days (~20 hours)
- Tests: 1 day (~8 hours)
- Documentation: 0.5 day (~4 hours)
- **Total**: ~3-4 days for solid, well-tested foundation

## Why Now?

Before v0.8.0 ships with Skein integration locked in, we should establish the architecture for multiple spec sources. Once Skein integration is live, refactoring becomes harder (more moving parts). This is a pre-release consolidation.

## References

- `WorkProvider` interface (existing) — parallel pattern for tracker abstraction
- `BoardProjector` interface (existing) — similar extension point architecture
- Beads: https://github.com/steveyegge/beads (reference implementation)
