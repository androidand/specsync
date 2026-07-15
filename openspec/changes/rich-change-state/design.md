# Design: Rich Change State Model

## Overview

This change introduces explicit data models for task progress, workflow stage, and stage derivation. It fixes the archived precedence bug and establishes the foundational schema for shared workflow metadata.

## Data Model

### Enums

**TaskProgress**:
```go
type TaskProgress string

const (
    TaskProgressNoTasks    TaskProgress = "no-tasks"      // no tasks.md
    TaskProgressNotStarted TaskProgress = "not-started"   // 0/N
    TaskProgressInProgress TaskProgress = "in-progress"   // 0 < X < N
    TaskProgressComplete   TaskProgress = "complete"      // N/N
)
```

**Stage**:
```go
type Stage string

const (
    StageBacklog   Stage = "backlog"
    StageBlocked   Stage = "blocked"
    StageActive    Stage = "active"
    StageInReview  Stage = "in-review"
    StageComplete  Stage = "complete"
    StageArchived  Stage = "archived"
)
```

Custom stages: any string matching `^[a-z0-9][a-z0-9-]{0,63}$`.

**StageSource**:
```go
type StageSource string

const (
    StageSourceDefault      StageSource = "default"
    StageSourceTasks        StageSource = "tasks"
    StageSourceMetadata     StageSource = "metadata"
    StageSourceLegacyStatus StageSource = "legacy-status"
    StageSourceFolder       StageSource = "folder"
)
```

### Metadata Schema

**ChangeMetadata** (from `.specsync.yaml`):
```go
type ChangeMetadata struct {
    Version  int   `yaml:"version"`
    Stage    *Stage `yaml:"stage,omitempty"`
    Priority *int  `yaml:"priority,omitempty"`
}
```

Fields are pointers so absence is distinguishable from zero values.

### Change Model (extend)

```go
type Change struct {
    // Existing fields...
    Dir           string
    Slug          string
    Title         string
    Body          string
    TasksMarkdown string
    Links         []Ref
    Archived      bool

    // New fields
    Progress    TaskProgress   // derived from TasksMarkdown
    Stage       Stage          // current workflow stage
    StageSource StageSource    // how Stage was derived
    Priority    *int           // optional 1–100; nil if unset
}
```

## File Layout

```
openspec/changes/my-feature/
├── proposal.md               ← OpenSpec
├── specs/                    ← OpenSpec
├── tasks.md                  ← OpenSpec
├── .specsync.yaml            ← NEW: committed workflow metadata
│   version: 1
│   stage: blocked
│   priority: 5
├── .status                   ← LEGACY: read-only fallback
└── .specsync/                ← cache (gitignored)
    ├── refs.json
    └── board.json (future)
```

## Derivation Algorithm

```go
func refreshState(c *Change) error {
    // Step 1: Always derive progress from tasks
    c.Progress = deriveTaskProgress(c.TasksMarkdown)

    // Step 2: Archived folder is final
    if c.Archived {
        c.Stage = StageArchived
        c.StageSource = StageSourceFolder
        return nil
    }

    // Step 3: Try explicit metadata
    metadata, err := loadChangeMetadata(c.Dir)
    if err != nil {
        return fmt.Errorf("invalid .specsync.yaml: %w", err)
    }
    if metadata.Stage != nil {
        c.Stage = *metadata.Stage
        c.StageSource = StageSourceMetadata
        return nil
    }

    // Step 4: Try legacy .status
    if legacyStage, ok := readLegacyStatus(c.Dir); ok {
        c.Stage = legacyStage
        c.StageSource = StageSourceLegacyStatus
        // Check for warning condition
        if metadata.Stage != nil && *metadata.Stage != legacyStage {
            warnConflict(c.Slug, *metadata.Stage, legacyStage)
        }
        return nil
    }

    // Step 5: Derive from task completion
    if c.Progress == TaskProgressComplete {
        c.Stage = StageComplete
        c.StageSource = StageSourceTasks
        return nil
    }

    // Step 6: Default
    c.Stage = StageActive
    c.StageSource = StageSourceDefault
    return nil
}
```

Key points:
- Archived check returns immediately; no subsequent rule applies
- Metadata wins over legacy; legacy is migration path
- Derivation is testable and transparent
- Errors are explicit (invalid YAML blocks the change)

## Task Progress Derivation

```go
func deriveTaskProgress(tasksMarkdown string) TaskProgress {
    if tasksMarkdown == "" {
        return TaskProgressNoTasks
    }

    total, completed := countCheckboxes(tasksMarkdown)
    if completed == 0 {
        return TaskProgressNotStarted
    }
    if completed == total {
        return TaskProgressComplete
    }
    return TaskProgressInProgress
}
```

## Metadata Parsing

```go
func loadChangeMetadata(dir string) (*ChangeMetadata, error) {
    path := filepath.Join(dir, ".specsync.yaml")
    data, err := os.ReadFile(path)
    if err != nil && os.IsNotExist(err) {
        return nil, nil  // file absent; no metadata
    }
    if err != nil {
        return nil, fmt.Errorf("read .specsync.yaml: %w", err)
    }

    var m ChangeMetadata
    if err := yaml.Unmarshal(data, &m); err != nil {
        return nil, fmt.Errorf("parse .specsync.yaml: %w", err)
    }

    if err := normalizeMetadata(&m); err != nil {
        return nil, err
    }
    return &m, nil
}

func normalizeMetadata(m *ChangeMetadata) error {
    if m.Version == 0 {
        m.Version = 1
    }

    if m.Version != 1 {
        return fmt.Errorf("unsupported .specsync.yaml version %d", m.Version)
    }

    if m.Stage != nil {
        if err := ValidateStage(*m.Stage); err != nil {
            return err
        }
    }

    if m.Priority != nil {
        if *m.Priority < 1 || *m.Priority > 100 {
            return fmt.Errorf("priority must be between 1 and 100; got %d", *m.Priority)
        }
    }

    return nil
}
```

## Validation Functions

```go
func ValidateStage(stage Stage) error {
    // Canonical stages always pass
    if stage == StageBacklog || stage == StageBlocked || /* ... */ {
        return nil
    }

    // Custom stages must match token pattern
    pattern := regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
    if !pattern.MatchString(string(stage)) {
        return fmt.Errorf(
            "invalid stage %q; must be canonical or match ^[a-z0-9][a-z0-9-]{0,63}$",
            stage,
        )
    }
    return nil
}

func IsCanonicalStage(stage Stage) bool {
    switch stage {
    case StageBacklog, StageBlocked, StageActive, StageInReview, StageComplete, StageArchived:
        return true
    default:
        return false
    }
}

func CanonicalStageOrder() []Stage {
    return []Stage{
        StageBacklog, StageBlocked, StageActive, StageInReview, StageComplete, StageArchived,
    }
}
```

## Backward Compatibility

### Reading .status

```go
func readLegacyStatus(dir string) (Stage, bool) {
    path := filepath.Join(dir, ".status")
    data, err := os.ReadFile(path)
    if err != nil {
        return "", false
    }
    stage := Stage(strings.TrimSpace(string(data)))
    return stage, true
}
```

No validation here; legacy stages are trusted as-is. Validation happens in `refreshState` if needed for specific operations.

### Warning on Conflict

```go
func warnConflict(slug string, yamlStage, statusStage Stage) {
    fmt.Fprintf(os.Stderr, 
        "warning: %s defines stage in both .specsync.yaml and legacy .status;\n"+
        "  using .specsync.yaml (%q)\n"+
        "  run `specsync set-stage %s auto` to migrate\n",
        slug, yamlStage, slug,
    )
}
```

## Error Handling Policy

### For `specsync changes` (read-only listing)

- Invalid `.specsync.yaml`: include change in output with diagnostic
- Continue processing other changes
- Exit code 0

### For `specsync sync` (projection to provider)

- Invalid `.specsync.yaml`: skip that change, print error
- Continue processing other changes
- Exit code non-zero

### For `set-stage` / `set-priority` (mutation)

- Invalid `.specsync.yaml`: fail with error
- Do not write any file
- Exit code non-zero
- Error message: "correct .specsync.yaml before setting state"

This ensures malformed committed state is visible and blocks mutation until corrected.

## Implementation Order

1. Add TaskProgress, Stage (extended), StageSource enums to change.go
2. Extend Change struct with Progress, Stage, StageSource, Priority *int
3. Write validation functions (ValidateStage, IsCanonicalStage, etc.)
4. Write loadChangeMetadata with YAML parsing and normalization
5. Write readLegacyStatus for backward compat
6. Implement refreshState with new algorithm and precedence
7. Fix archived precedence bug (return immediately)
8. Add error handling: return error from LoadChange on invalid metadata
9. Write tests: all precedence paths, custom stages, validation, conflict warning, archived finality
10. Update change_test.go with comprehensive coverage

## Notes

- No CLI commands here (separate change: change-status-cli)
- No board projection changes (separate change: board-state-reconciliation)
- Priority loading is defensive; paired with set-priority command later
- `.specsync.yaml` can grow fields in future without breaking this model (version is explicit)
