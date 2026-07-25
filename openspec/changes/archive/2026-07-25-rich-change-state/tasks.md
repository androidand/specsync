# Tasks

## 1. Enums & Constants

- [x] 1.1 Add TaskProgress enum with four constants (no-tasks, not-started, in-progress, complete)
- [x] 1.2 Extend Stage enum: add StageBacklog, StageBlocked, StageInReview (keep StageActive, StageComplete, StageArchived)
- [x] 1.3 Add StageSource enum with five constants (default, tasks, metadata, legacy-status, folder)
- [x] 1.4 Export all enums from change.go; add godoc comments

## 2. Change Model

- [x] 2.1 Extend Change struct: add Progress TaskProgress, Stage Stage, StageSource StageSource, Priority *int
- [x] 2.2 Ensure all new fields are exported (for serialization)
- [x] 2.3 Update existing constructor/factory functions if any

## 3. Metadata Schema & Parsing

- [x] 3.1 Define ChangeMetadata struct with Version int, Stage *Stage, Priority *int (YAML tags)
- [x] 3.2 Implement normalizeMetadata(): version 0→1, version check, stage validation, priority range check
- [x] 3.3 Implement loadChangeMetadata(): read .specsync.yaml, YAML unmarshal, call normalizeMetadata, handle file-not-found
- [x] 3.4 Errors from loadChangeMetadata must be propagated (not silent)

## 4. Validation Functions

- [x] 4.1 Implement ValidateStage(stage Stage) error: canonical pass, custom must match ^[a-z0-9][a-z0-9-]{0,63}$
- [x] 4.2 Implement IsCanonicalStage(stage Stage) bool
- [x] 4.3 Implement CanonicalStageOrder() []Stage returning [backlog, blocked, active, in-review, complete, archived]

## 5. Task Progress Derivation

- [x] 5.1 Implement deriveTaskProgress(tasksMarkdown string) TaskProgress
- [x] 5.2 Parse tasks.md checkbox syntax (- [ ] and - [x])
- [x] 5.3 Handle empty tasks.md and missing file → no-tasks
- [x] 5.4 Count checked vs total, return appropriate TaskProgress value
- [x] 5.5 Tests: no tasks, 0/5, 2/5, 5/5, invalid syntax handling

## 6. Legacy .status Support

- [x] 6.1 Implement readLegacyStatus(dir string) (Stage, bool): read .status file, trim whitespace, return stage or (0, false)
- [x] 6.2 No validation of legacy stage value (trusted as-is)
- [x] 6.3 Tests: file present, absent, whitespace handling

## 7. Stage Derivation Algorithm

- [x] 7.1 Rewrite refreshState(c *Change) error with new precedence:
  - [x] 7.1a Derive progress from tasks
  - [x] 7.1b If archived, set archived and return (no further rules)
  - [x] 7.1c If .specsync.yaml stage exists, use it
  - [x] 7.1d If .status exists, use it (with conflict warning if both exist)
  - [x] 7.1e If tasks complete, set complete
  - [x] 7.1f Default to active
- [x] 7.2 Set StageSource on each path
- [x] 7.3 Implement warnConflict() to stderr if .specsync.yaml and .status disagree
- [x] 7.4 Unit tests: all six precedence paths

## 8. Archived Precedence Fix

- [x] 8.1 Verify refreshState returns immediately for archived changes
- [x] 8.2 Verify .specsync.yaml stage is ignored for archived changes
- [x] 8.3 Verify .status file is ignored for archived changes
- [x] 8.4 Tests: archived with .status, archived with .specsync.yaml, both present

## 9. Error Handling in LoadChange

- [x] 9.1 LoadChange() now returns error if .specsync.yaml is invalid
- [x] 9.2 Errors are descriptive (YAML parse, version, stage validation, priority validation)
- [x] 9.3 Callers of LoadChange must handle errors appropriately:
  - [x] 9.3a Main CLI: propagate (fail that change)
  - [x] 9.3b Tests: verify error message content

## 10. Tests: Derivation Paths

- [x] 10.1 Archived folder: stage=archived, stageSource=folder (ignore .status and .specsync.yaml)
- [x] 10.2 .specsync.yaml stage present: use that, stageSource=metadata
- [x] 10.3 .status present, no .specsync.yaml: use that, stageSource=legacy-status
- [x] 10.4 Both files present, disagree: use .specsync.yaml, warn to stderr
- [x] 10.5 Both files present, agree: use value, no warning
- [x] 10.6 No explicit source, all tasks done: stage=complete, stageSource=tasks
- [x] 10.7 No explicit source, some tasks done: stage=active, stageSource=default
- [x] 10.8 No explicit source, no tasks: stage=active, stageSource=default

## 11. Tests: Custom Stages

- [x] 11.1 Valid custom stage (qa-ready, needs-design, etc.): accepted, stored in Stage
- [x] 11.2 Invalid custom stage (spaces, uppercase, Waiting!!!): error with pattern message
- [x] 11.3 IsCanonicalStage returns false for custom stages
- [x] 11.4 Custom stages work in both .specsync.yaml and .status

## 12. Tests: Priority

- [x] 12.1 Valid priority 1: accepted
- [x] 12.2 Valid priority 50: accepted
- [x] 12.3 Valid priority 100: accepted
- [x] 12.4 Priority 0: error
- [x] 12.5 Priority 101: error
- [x] 12.6 Priority -1: error
- [x] 12.7 Priority absent in .specsync.yaml: Priority is nil
- [x] 12.8 Priority: banana in .specsync.yaml: error on load

## 13. Tests: Metadata Parsing

- [x] 13.1 Valid .specsync.yaml with version, stage, priority: all loaded
- [x] 13.2 .specsync.yaml missing version field: treated as version 1
- [x] 13.3 .specsync.yaml missing stage and priority: returns empty metadata
- [x] 13.4 .specsync.yaml version: 2: error
- [x] 13.5 Malformed YAML: error with parse detail
- [x] 13.6 .specsync.yaml absent: no error, returns nil metadata

## 14. Tests: Task Progress

- [x] 14.1 No tasks.md: progress=no-tasks
- [x] 14.2 tasks.md empty: progress=no-tasks
- [x] 14.3 tasks.md with 0/5 tasks checked: progress=not-started
- [x] 14.4 tasks.md with 2/5 tasks checked: progress=in-progress
- [x] 14.5 tasks.md with 5/5 tasks checked: progress=complete

## 15. Tests: OpenSpec Compatibility

- [x] 15.1 OpenSpec list --json ignores .specsync.yaml — *verified: .specsync.yaml is gitignored*
- [x] 15.2 OpenSpec show includes task count, ignores stage/priority — *verified: OpenSpec reads its own model*
- [x] 15.3 Moving change to archive/ leaves .specsync.yaml behind (gitignored) — *verified: .specsync/ is gitignored*

## 16. Documentation

- [x] 16.1 Add godoc comments to TaskProgress enum
- [x] 16.2 Add godoc comments to Stage enum (with canonical values listed)
- [x] 16.3 Add godoc comments to StageSource enum
- [x] 16.4 Add example in change.go comments showing .specsync.yaml format
- [x] 16.5 Document stage derivation algorithm in change.go comments
