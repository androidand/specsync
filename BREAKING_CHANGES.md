# Breaking Changes in v0.7.0+

## Overview

The v0.7.0+ release introduces workflow state management and priority dispatch. Most changes are **backwards compatible**, but there are important considerations for:

1. **Programmatic users** (using specsync as a library)
2. **Integration points** (custom providers, workflows)
3. **Archived changes** (immutable by design)

---

## Breaking Changes (By Severity)

### 🔴 **BREAKING: Change struct has new required fields**

**Impact**: If you unmarshal Change structs from JSON or parse them, new fields must be handled.

**What changed**:
- Added `Progress` field (TaskProgress enum)
- Added `Stage` field (Stage enum)
- Added `StageSource` field (StageSource enum)
- Added `Priority` field (*int, nullable)

**What this means**:
```go
// OLD: This still works
type MyCustomChange struct {
	Slug string
	Body string
}

// NEW: These fields are now present on specsync.Change
type specsync.Change struct {
	// ... existing fields ...
	Progress    TaskProgress  // NEW
	Stage       Stage         // NEW
	StageSource StageSource   // NEW
	Priority    *int          // NEW
}
```

**Migration strategy**:
- If you're reading specsync.Change structs, the new fields are always populated
- If you're serializing them, include the new fields in your format
- Empty/default values are handled safely (nil priority, backlog stage)

---

### 🟡 **BREAKING: LoadChanges() now reads .specsync/metadata.json**

**Impact**: `LoadChanges()` behavior changes when `.specsync/metadata.json` exists.

**What changed**:
- Priority and stage are now loaded from `.specsync/metadata.json`
- Stage is derived with precedence: archived → metadata → legacy-status → task-derived → default
- Changes without metadata.json default to priority=nil, stage=backlog

**What this means**:
```bash
# BEFORE: Priority and stage were always nil
specsync pull issue-123  # Change has Priority=nil, Stage=active (derived)

# AFTER: Priority and stage come from .specsync/metadata.json if present
specsync set-priority my-change 85
# Next time you call LoadChanges(), Priority=85
```

**Migration strategy**:
- Safe for existing code: missing metadata.json doesn't error
- Safe for existing data: changes without metadata.json work as before
- No action required unless you were ignoring .specsync/ directory

---

### 🟡 **BREAKING: Board state persistence in .specsync/board.json**

**Impact**: Board reconciliation now requires additional state files.

**What changed**:
- `.specsync/board.json` (gitignored) stores board binding for three-way merge
- Detects human board moves and doesn't clobber them

**What this means**:
```bash
# BEFORE: No human-move detection
# Next sync would drag card back to "In Progress" even if human moved to "Done"

# AFTER: Three-way merge respects human moves
# If human moved card, status_skipped logs reason and doesn't update
```

**Migration strategy**:
- `.specsync/board.json` is gitignored (safe to regenerate)
- First sync after upgrade will create binding automatically
- No action required, but behavior changes subtly (better)

---

### 🟡 **BREAKING: CLI flag changes**

#### New flag: `--spec`

**Impact**: New optional flag on all commands.

```bash
# ALL these now support --spec flag
specsync sync --spec openspec          # default
specsync pull --spec openspec
specsync changes --spec openspec
specsync set-stage --spec openspec
specsync set-priority --spec openspec
```

**Migration strategy**:
- Flag is optional, defaults to "openspec"
- Existing scripts work unchanged
- No breaking change, purely additive

#### New commands: `set-stage`, `set-priority`

**Impact**: Two new CLI commands added.

```bash
specsync set-stage my-change active
specsync set-priority my-change 85
```

**Migration strategy**:
- Purely additive, doesn't break existing commands
- No action required

---

### 🟢 **NON-BREAKING: Archived changes are now immutable**

**Impact**: `set-stage` and `set-priority` reject archived changes.

**What changed**:
```bash
# BEFORE: Could change archived change's priority
specsync set-priority archived-change 50  # ✓ worked

# AFTER: Archived changes are immutable
specsync set-priority archived-change 50  # ✗ error: cannot mutate archived change
```

**Why this is non-breaking**:
- Archived changes are intended to be immutable (by design)
- Any code that tried to mutate archived changes was already problematic
- Error message is clear: "cannot mutate archived change"

**Migration strategy**:
- If your workflow mutates archived changes, unarchive first
- This is a feature, not a bug (prevents accidental mutation)

---

## Configuration Changes

### New: `.specsync/metadata.json`

**Format**:
```json
{
  "version": 1,
  "stage": "active",
  "priority": 85
}
```

**Locations**:
- `openspec/changes/<slug>/.specsync/metadata.json` (committed)
- `openspec/changes/archive/<slug>/.specsync/metadata.json` (committed)

**Backwards compatibility**:
- Optional: missing file means default priority/stage
- Safe to git-commit (not sensitive)
- No conflicts with existing .specsync/ structure

---

### New: `.specsync/board.json`

**Format**:
```json
{
  "version": 1,
  "bindings": {
    "github:owner/5": {
      "provider": "github",
      "project_id": "...",
      "item_id": "...",
      "local_stage_base": "active",
      "remote_option_id_base": "...",
      "synced_at": "2026-07-16T..."
    }
  }
}
```

**Locations**:
- `openspec/changes/<slug>/.specsync/board.json` (gitignored)
- Regenerated on each sync

**Backwards compatibility**:
- Gitignored: safe to not commit
- Disposable: deleted safely and recreated
- Purely for reconciliation logic

---

## API Changes (for library users)

### New public types

```go
type Stage string
type TaskProgress string
type StageSource string
type ChangeMetadata struct {
	Version  int
	Stage    string
	Priority *int
}
```

### New public functions

```go
func ValidateStage(stage Stage) error
func IsCanonicalStage(stage Stage) bool
func CanonicalStageOrder() []Stage
func RefreshState(change *Change) error
func LoadChangeBySlug(openspecDir, slug string) (*Change, error)
func LoadBoardState(changeDir string) (BoardState, error)
func SaveBoardState(changeDir string, state BoardState) error
```

### Changed behavior

```go
// OLD: LoadChanges() returned only basic fields
changes, err := LoadChanges(openspecDir)

// NEW: LoadChanges() now populates Priority, Stage, StageSource
// via .specsync/metadata.json and derivation logic
changes, err := LoadChanges(openspecDir)
// Each change now has:
//   Priority: loaded from metadata.json (may be nil)
//   Stage: derived with multi-level precedence
//   StageSource: shows where Stage came from
```

---

## Migration Checklist

### For users of specsync CLI

- [ ] Update to v0.7.0+
- [ ] No action needed (fully backwards compatible)
- [ ] Optionally: Use `specsync set-priority` to prioritize changes

### For users of specsync as a library

- [ ] Update to v0.7.0+
- [ ] Check if you handle `Priority`, `Stage`, `StageSource` fields
- [ ] Update serialization (if you serialize Change structs)
- [ ] Consider if archived-change immutability affects your code
- [ ] No action needed if you don't read these fields

### For custom providers/integrations

- [ ] Update to v0.7.0+
- [ ] If you implement WorkProvider, check if changes have new fields
- [ ] BoardBinding struct is new (used for three-way merge)
- [ ] No changes to provider interface, but behavior is subtly different

### For scripts/automation

- [ ] Existing shell scripts work unchanged
- [ ] Optionally adopt new `set-stage` and `set-priority` commands
- [ ] No action needed

---

## Deprecations (None announced)

No deprecations yet. The API is stable as of v0.7.0.

---

## Future considerations

### Phase 3.5: Board reconciliation inbound read

**Potential breaking change**: Reading board status back into specsync state (inverse of current sync).

- May add `BoardStatus` field to Change
- May affect how `Stage` is derived (include board state in precedence)
- Requires careful thought about conflict resolution
- Not scheduled, needs design review

### Phase 7: Beads format support

**Potential breaking change**: When Beads format is supported.

- `--spec beads` will work alongside `--spec openspec`
- Beads format has different change structure
- May require API changes to abstraction layer
- Backwards compatible at CLI level (flag is optional)

---

## Decision Framework for This Release

### What to consider before breaking changes

1. **Is there a safe default?** → Make it automatic (metadata.json loading)
2. **Is the old behavior problematic?** → Add safety gates (immutable archived)
3. **Can it be additive?** → Add new fields, don't remove old ones
4. **Is the change localized?** → Gitignored files (board.json) can be risky-free

### Why these changes were accepted

1. ✅ `Stage` and `Priority` fields on Change: Non-breaking (additive)
2. ✅ metadata.json reading: Non-breaking (optional file)
3. ✅ board.json state: Non-breaking (gitignored, regenerated)
4. ✅ Archived immutability: Non-breaking (fix for user error)
5. ⚠️ LoadChanges behavior: Subtle change, backwards compatible

---

## Questions & Answers

**Q: Will my old scripts break?**
A: No. All CLI commands work as before. New commands are optional.

**Q: Do I need to migrate my .specsync/ directories?**
A: No. Missing metadata.json and board.json default safely.

**Q: What happens if I commit .specsync/board.json by accident?**
A: It's safe but wasteful. Add to .gitignore. It will be regenerated.

**Q: Can I use specsync v0.7.0 with repos that have v0.6.0 changes?**
A: Yes, fully compatible. Changes without metadata.json default safely.

**Q: Is the three-way merge stable?**
A: Yes, tested extensively. Human-move detection has no false positives.

---

## Version Timeline

| Version | Feature | Breaking |
|---------|---------|----------|
| v0.6.0  | changelog, release-plan | No |
| **v0.7.0** | **Priority, stages, board reconciliation** | **Minimal** |
| v0.8.0  | Board inbound read (planned) | Maybe |
| v1.0.0  | Stable API guarantee | TBD |
