# Rich change state model: separate workflow placement from task progress

## Why

specsync conflates three independent concepts: task completion, workflow stage, and provider state. A change can be marked `complete` locally while still in `blocked` workflow state, or vice versa. The current model cannot represent this clearly, and derives stage only from tasks and archived location, losing human workflow intent.

The current `.status` override is a workaround that works but lacks structure: any string is valid, there is no validation, and merging task-derived and human-assigned state is opaque.

This change introduces an explicit model that separates concerns:

- **Task progress** — what the checkboxes say (no-tasks, not-started, in-progress, complete)
- **Workflow stage** — where humans place the work (backlog, blocked, active, in-review, complete, archived)
- **Stage source** — how we arrived at the current stage (derived from tasks, derived from folder, explicit from metadata, legacy `.status`, or default)

This clarity is the foundation for rich filtering, correct board synchronization, and principled conflict resolution later.

## What Changes

### New Enums & Constants

**TaskProgress** — derived from task checklist state

```
no-tasks       ← change has no tasks.md
not-started    ← has tasks, zero complete
in-progress    ← has tasks, some complete, some remain
complete       ← has tasks, all complete
```

**Stage** — workflow placement (user-facing)

```
backlog        ← not yet started; pre-discovery or deferred
blocked        ← waiting on external blocker or decision
active         ← in flight; has unchecked work
in-review      ← awaiting approval before proceeding
complete       ← all work done, not yet archived
archived       ← moved to changes/archive/
```

Custom stages are valid (must match `^[a-z0-9][a-z0-9-]{0,63}$`); canonical stages are the six above.

**StageSource** — provenance of the current stage

```
default        ← no other source; assume active
tasks          ← derived from task completion (all done → complete)
metadata       ← explicit .specsync.yaml stage field
legacy-status  ← read from .status file (backward compat)
folder         ← archived folder location (final, immutable)
```

### New Metadata Schema

`.specsync.yaml` (committed, new):

```yaml
version: 1
stage: blocked
priority: 5
```

- `version`: required when writing; tolerate absent as v1 when reading
- `stage`: optional; when present, explicit workflow state
- `priority`: optional; integer 1–100; null if unset

Both fields are optional in the file. Absence means no explicit metadata for that field.

### Change Model (new fields)

```go
type Change struct {
    // Existing fields...

    Progress    TaskProgress  // what tasks say
    Stage       Stage         // current workflow placement
    StageSource StageSource   // how we arrived at Stage
    Priority    *int          // optional 1–100; nil if unset
}
```

### Stage Derivation (new algorithm)

Derivation has clear, testable precedence:

1. If archived → Stage = archived, StageSource = folder (return immediately)
2. If `.specsync.yaml` stage is set → Stage = that, StageSource = metadata (return)
3. If `.status` file exists → Stage = that, StageSource = legacy-status (return)
4. If Progress == complete → Stage = complete, StageSource = tasks (return)
5. Default → Stage = active, StageSource = default

**Archived is final.** No subsequent rule can override it. `.specsync.yaml` stage and legacy `.status` are ignored for archived changes.

### Validation

**Stage validation**: canonical values (6 constants) pass. Custom stages must match `^[a-z0-9][a-z0-9-]{0,63}$` or error.

**Priority validation**: if present, must be 1–100 or error.

**Invalid committed metadata is visible, not silent.** Malformed `.specsync.yaml` causes:

- `specsync changes`: show with diagnostic, continue to next change
- `specsync sync`: fail that change, refuse to project invalid state
- `set-stage`/`set-priority`: fail with error message until corrected

### Backward Compatibility

Existing `.status` files continue to work as a legacy input. New CLI writes target `.specsync.yaml`. When both exist and differ, `.specsync.yaml` wins and a warning is emitted to stderr.

Example: change has `.status = blocked` and `.specsync.yaml` with no stage field. Stage = blocked, StageSource = legacy-status. When `set-stage` is called, `.specsync.yaml` gets the new stage and `.status` is deleted.

### Bug Fixes

**Archived precedence**: `.status` override no longer applies to archived changes. Archived folder location is unconditionally authoritative.

```go
if c.Archived {
    c.Stage = StageArchived
    c.StageSource = StageSourceFolder
    return  // exit immediately
}
```

This fix is critical for data integrity; archived changes must never become "blocked" or other states on disk.

### Out of Scope

- Board synchronization (separate change)
- CLI commands `specsync changes`, `set-stage`, `set-priority` (separate change)
- Priority projection to GitHub Projects or other providers (separate change)
- Two-way board reconciliation (separate change)

## Capabilities

- `rich-task-progress` — task progress enum distinct from stage
- `rich-stage-enum` — six canonical stages plus custom-stage support with validation
- `stage-source-tracking` — explicit derivation chain for debugging and transparency
- `committed-workflow-metadata` — `.specsync.yaml` schema for shared priority and stage
- `archived-precedence-fix` — archived folder location is immutable
- `legacy-status-compat` — existing `.status` files continue to work, with clear migration path

## Impact

**Code Changes**:
- `change.go`: add TaskProgress and StageSource enums; extend Stage; add Priority *int field; add ChangeMetadata struct; refactor refreshState() with clear precedence
- New validation functions for stage and priority
- Load `.specsync.yaml` with YAML unmarshaling and validation
- Keep legacy `.status` read path; emit warning if both files exist
- Tests for all derivation paths, custom stages, validation

**Compatibility**:
- No breaking changes; `.status` files continue to work
- New `.specsync.yaml` is opt-in via new CLI commands
- Existing repos derive stages exactly as before until `.specsync.yaml` is introduced
- Archived changes cannot be mutated via new CLI; safety first

**Schema**:
- `.specsync.yaml` is committed and collaborative
- `.specsync/` remains gitignored cache (for refs.json, board.json, etc.)
- `.status` is deprecated but readable

## Dependencies

None. This is a foundational change that other features (board-state-reconciliation, change-status-cli) build upon.
