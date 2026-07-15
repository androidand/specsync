# Design: Change Status CLI

## Overview

Three new subcommands provide read and write access to change state:

- `specsync changes` — list and filter changes (read-only)
- `specsync set-stage` — set workflow stage (mutation)
- `specsync set-priority` — set priority (mutation)

## Command: specsync changes

**Subcommand Structure**:
```
specsync changes [flags]
```

**Flags**:
```
-stage <list>      Filter by stages (comma-separated; default: all)
-sort <order>      Sort: priority or stage (default: stage order)
-json              Output as JSON (default: table)
-openspec <dir>    OpenSpec directory (default: ./openspec)
```

**Implementation**:

1. Load all changes via LoadChanges()
2. Apply filters (`-stage` flags)
3. Sort by canonical-stage-order → priority → slug (or by priority within stage if `-sort priority`)
4. Render table or JSON
5. Exit code 0 if all changes load successfully; if any have diagnostics, still exit 0 (reporting issues, not blocking)

**Table Rendering**:
```
STAGE      PRIORITY  SLUG                     PROGRESS        TASKS  TITLE
─────────────────────────────────────────────────────────────────────────────
backlog    2         add-dark-mode            not-started     0/5    Add dark...
```

Use tabwriter or simple string formatting. Truncate Title to 60 chars if needed.

**JSON Schema**:
```json
{
  "slug": "add-dark-mode",
  "title": "Add dark-mode toggle",
  "stage": "backlog",
  "canonicalStage": true,
  "stageSource": "metadata",
  "priority": 2,
  "taskProgress": "not-started",
  "completedTasks": 0,
  "totalTasks": 5,
  "archived": false,
  "diagnostics": []
}
```

`priority` is null (not 0) when unset. `diagnostics` is array of diagnostic objects (see below).

**Diagnostic Schema**:
```json
{
  "code": "unmapped-stage",
  "severity": "warning",
  "message": "Custom stage \"qa-ready\" has no GitHub Projects mapping"
}
```

Codes: `unmapped-stage`, `invalid-stage`, `invalid-priority`, `parse-error`, etc.

## Command: specsync set-stage

**Subcommand Structure**:
```
specsync set-stage <slug> <stage> [reason]
```

**Arguments**:
- `slug`: change slug (required)
- `stage`: new stage value or "auto" (required)
- `reason`: optional description (logged to stdout)

**Implementation**:

1. Validate slug (no path traversal; must match `^[a-z0-9][a-z0-9_-]+$` or similar)
2. Locate change directory
3. If archived, reject with error
4. If stage != "auto":
   - Validate stage (canonical or custom pattern)
5. Load current .specsync.yaml (if exists)
6. Load legacy .status (if exists)
7. If stage == "auto":
   - Remove stage from metadata (or delete file if only field)
   - Delete .status
8. Else:
   - Set stage in metadata
   - Delete .status (migration)
9. If metadata is now empty, delete .specsync.yaml
10. Write atomically (temp file + rename)

**Atomic Write Pattern**:
```go
tempPath := dir + "/.specsync.yaml.tmp"
if err := ioutil.WriteFile(tempPath, data, 0644); err != nil {
    return err
}
if err := os.Rename(tempPath, yamlPath); err != nil {
    os.Remove(tempPath)
    return err
}
```

**Error Handling**:
- Slug not found: "change not found: my-change"
- Archived: "cannot mutate archived change my-change"
- Invalid stage: "invalid stage \"STAGE NAME\"; must match ^[a-z0-9][a-z0-9-]{0,63}$"
- Malformed .specsync.yaml: "cannot update change; fix .specsync.yaml: ... first"

## Command: specsync set-priority

**Subcommand Structure**:
```
specsync set-priority <slug> <number|unset>
```

**Arguments**:
- `slug`: change slug (required)
- `number`: integer 1–100 or "unset" (required)

**Implementation**:

1. Validate slug (same as set-stage)
2. Locate change directory
3. If number != "unset":
   - Parse as integer
   - Validate 1 ≤ number ≤ 100
4. Load current .specsync.yaml (if exists)
5. If number == "unset":
   - Remove priority from metadata (or delete file if only field)
6. Else:
   - Set priority in metadata
7. If metadata is now empty, delete .specsync.yaml
8. Write atomically

**Error Handling**:
- Slug not found: "change not found: my-change"
- Priority out of range: "priority must be between 1 and 100; got 150"
- Not a number: "invalid priority: not-a-number"
- Malformed .specsync.yaml: (same as set-stage)

## Integration with rich-change-state

- `specsync changes` reads Change.Stage, Change.Priority, Change.Progress, Change.StageSource
- `set-stage`/`set-priority` mutate `.specsync.yaml` directly; refreshState() is called on next load
- No circular dependency; both are in separate source files

## Path Safety

```go
func validateSlug(slug string) error {
    if slug == "" || strings.Contains(slug, "/") || strings.Contains(slug, "..") {
        return fmt.Errorf("invalid slug: %s", slug)
    }
    // Additional checks
    if !regexp.MustCompile(`^[a-z0-9][a-z0-9_-]+$`).MatchString(slug) {
        return fmt.Errorf("invalid slug format: %s", slug)
    }
    return nil
}
```

## Implementation Order

1. Implement table rendering helper (formatChangeTable)
2. Implement JSON marshaling helpers
3. Implement `specsync changes` command:
   - LoadChanges()
   - Filter by -stage
   - Sort
   - Render
4. Implement `specsync set-stage` command:
   - validateSlug
   - Load/mutate metadata
   - Atomic write
5. Implement `specsync set-priority` command:
   - validateSlug
   - Load/mutate metadata
   - Atomic write
6. Add help/usage strings
7. Tests: all commands, filtering, sorting, mutations, error cases, path safety

## Error Recovery

If `.specsync.yaml` becomes corrupted during operation:

- Read: fail fast, report to user
- Mutation: check malformed before writing; refuse to proceed
- Recovery: user must manually fix YAML or delete the file

We do not attempt auto-repair.
