# Change status CLI: query and mutate workflow state locally

## Why

Teams need visibility into workflow state without external tooling. "What's blocked?" and "What should we prioritize?" are common questions, but answering them requires scanning the filesystem or running external commands.

This change introduces a command-line interface for querying and managing change state locally. It surfaces the rich state model (stage, priority, task progress) through a readable table and JSON output, and provides mutation commands (`set-stage`, `set-priority`) to adjust workflow placement and priority directly.

The CLI works offline, requires no external service, and operates on the source of truth: local OpenSpec changes.

## What Changes

### New `specsync changes` Command

**Overview**: List OpenSpec changes with stage and priority, filterable and sortable.

**Usage**:
```bash
specsync changes                              # all changes, grouped by stage
specsync changes -stage backlog,blocked       # filter by stages
specsync changes -sort priority               # sort by priority within stage
specsync changes -stage active -sort priority # filter and sort
specsync changes -json                        # machine-readable output
specsync changes -json -stage backlog         # filtered JSON
```

**Table Output** (default):
```
STAGE      PRIORITY  SLUG                     PROGRESS        TASKS  TITLE
─────────────────────────────────────────────────────────────────────────────
backlog    2         add-dark-mode            not-started     0/5    Add dark mode
backlog    5         performance-audit        not-started     0/12   Audit rendering

blocked    1         waiting-on-auth          in-progress     3/8    Implement OIDC

active     3         refactor-api-routes      in-progress     5/10   Refactor routes
active     -         critical-fix             complete        7/7    Fix auth bug

in-review  1         security-hardening       complete        12/12  Harden sessions

complete   -         spec-issue-linker        complete        15/15  Link specs/issues
complete   -         v0.5-release-prep        complete        10/10  Prepare v0.5

archived   -         old-feature              complete        8/8    Old removed feature
```

**JSON Output** (`-json` flag):
```json
[
  {
    "slug": "add-dark-mode",
    "title": "Add dark-mode toggle",
    "stage": "backlog",
    "canonicalStage": true,
    "stageSource": "override",
    "priority": 2,
    "taskProgress": "not-started",
    "completedTasks": 0,
    "totalTasks": 5,
    "archived": false,
    "diagnostics": []
  },
  {
    "slug": "waiting-on-auth",
    "title": "Implement OIDC",
    "stage": "qa-ready",
    "canonicalStage": false,
    "stageSource": "legacy-status",
    "priority": null,
    "taskProgress": "in-progress",
    "completedTasks": 3,
    "totalTasks": 8,
    "archived": false,
    "diagnostics": [
      {
        "code": "unmapped-stage",
        "severity": "warning",
        "message": "Custom stage \"qa-ready\" has no GitHub Projects mapping"
      }
    ]
  }
]
```

**Default Sorting**: canonical stage order → priority (1 first, unset last) → slug.

**Flags**:
- `-stage <list>`: filter by one or more stages (comma-separated; case-sensitive)
- `-sort <order>`: sort by priority or stage (default: stage order)
- `-json`: output as JSON array
- `-openspec <dir>`: non-default openspec path (default: "./openspec")

### New `specsync set-stage` Command

**Overview**: Explicitly set a change's workflow stage in `.specsync.yaml`.

**Usage**:
```bash
specsync set-stage <slug> <stage> [reason]
```

**Behavior**:
- Looks up `openspec/changes/<slug>/`
- Validates slug exists and is not archived
- Writes stage to `.specsync.yaml` (creates file if missing)
- Deletes legacy `.status` file if it exists (migration)
- Optional reason: logged to stdout (or `.specsync/stage-history` if implemented later)
- Atomic write; no partial state on error

**Examples**:
```bash
specsync set-stage add-dark-mode active
specsync set-stage waiting-on-auth blocked "Waiting on backend API contract"
specsync set-stage security-hardening in-review
```

**Special Values**:
```bash
specsync set-stage my-change auto
# Deletes .specsync.yaml stage and .status file (restores task-derived behavior)
```

**Error Cases**:
- Slug not found: error message
- Change is archived: error (cannot mutate archived changes)
- Invalid stage value: error (must match canonical or custom pattern)
- Malformed `.specsync.yaml` already exists: error (must be corrected first)

### New `specsync set-priority` Command

**Overview**: Set a change's priority (1–100) in `.specsync.yaml`.

**Usage**:
```bash
specsync set-priority <slug> <number>
```

**Behavior**:
- Looks up `openspec/changes/<slug>/`
- Validates priority is 1–100
- Writes priority to `.specsync.yaml` (creates file if missing)
- Atomic write
- Does not require change to be active or non-archived

**Examples**:
```bash
specsync set-priority add-dark-mode 2
specsync set-priority critical-fix 1
specsync set-priority future-work 50
```

**Special Values**:
```bash
specsync set-priority my-change unset
# Removes priority from .specsync.yaml (no priority = no prioritization)
```

**Error Cases**:
- Slug not found: error message
- Priority out of range (0, 101, etc.): error (must be 1–100)
- Priority: not-a-number: error
- Malformed `.specsync.yaml` already exists: error (must be corrected first)

## Capabilities

- `change-status-query`: list and filter changes by stage, priority, and progress
- `change-status-json`: machine-readable output with diagnostics
- `change-status-mutate`: set-stage and set-priority commands with validation
- `legacy-status-migration`: automatic .status → .specsync.yaml migration
- `auto-restore-behavior`: `set-stage auto` removes explicit overrides

## Compatibility & Integration

- **OpenSpec**: `.specsync.yaml` is not parsed by OpenSpec; fully transparent
- **rich-change-state**: depends on rich-change-state for TaskProgress, StageSource, validated Stage
- **Existing repos**: work as-is; new CLI commands are opt-in
- **JSON schema**: stable for agent use; includes diagnostics for clarity

## Out of Scope

- Two-way board synchronization (separate change: board-state-reconciliation)
- Automatic stage transitions (deferred)
- Dependency blocking
- Personal worksets or filtering by owner
- Shell completion (can add later)

## Impact

**Code Changes**:
- `cmd/specsync/main.go`: add three new subcommands (changes, set-stage, set-priority)
- Validation and mutation logic for .specsync.yaml
- Table and JSON formatting for changes output
- Error handling for all edge cases
- Tests: command parsing, filtering, sorting, JSON output, mutation, archived rejection

**UX**:
- New commands are self-discoverable (`specsync changes --help`)
- Table output is readable; JSON is machine-parseable
- Clear error messages guide users to fix malformed state

**Breaking Changes**: None. Existing commands and workflows are unaffected.

## Dependencies

- Depends on `rich-change-state` for Stage, TaskProgress, StageSource, ChangeMetadata
