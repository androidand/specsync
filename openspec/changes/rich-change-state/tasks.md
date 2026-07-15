# Tasks

## 1. Enums & Constants

- [ ] 1.1 Add TaskProgress enum with four constants (no-tasks, not-started, in-progress, complete)
- [ ] 1.2 Extend Stage enum: add StageBacklog, StageBlocked, StageInReview (keep StageActive, StageComplete, StageArchived)
- [ ] 1.3 Add StageSource enum with five constants (default, tasks, metadata, legacy-status, folder)
- [ ] 1.4 Export all enums from change.go; add godoc comments

## 2. Change Model

- [ ] 2.1 Extend Change struct: add Progress TaskProgress, Stage Stage, StageSource StageSource, Priority *int
- [ ] 2.2 Ensure all new fields are exported (for serialization)
- [ ] 2.3 Update existing constructor/factory functions if any

## 3. Metadata Schema & Parsing

- [ ] 3.1 Define ChangeMetadata struct with Version int, Stage *Stage, Priority *int (YAML tags)
- [ ] 3.2 Implement normalizeMetadata(): version 0→1, version check, stage validation, priority range check
- [ ] 3.3 Implement loadChangeMetadata(): read .specsync.yaml, YAML unmarshal, call normalizeMetadata, handle file-not-found
- [ ] 3.4 Errors from loadChangeMetadata must be propagated (not silent)

## 4. Validation Functions

- [ ] 4.1 Implement ValidateStage(stage Stage) error: canonical pass, custom must match ^[a-z0-9][a-z0-9-]{0,63}$
- [ ] 4.2 Implement IsCanonicalStage(stage Stage) bool
- [ ] 4.3 Implement CanonicalStageOrder() []Stage returning [backlog, blocked, active, in-review, complete, archived]

## 5. Task Progress Derivation

- [ ] 5.1 Implement deriveTaskProgress(tasksMarkdown string) TaskProgress
- [ ] 5.2 Parse tasks.md checkbox syntax (- [ ] and - [x])
- [ ] 5.3 Handle empty tasks.md and missing file → no-tasks
- [ ] 5.4 Count checked vs total, return appropriate TaskProgress value
- [ ] 5.5 Tests: no tasks, 0/5, 2/5, 5/5, invalid syntax handling

## 6. Legacy .status Support

- [ ] 6.1 Implement readLegacyStatus(dir string) (Stage, bool): read .status file, trim whitespace, return stage or (0, false)
- [ ] 6.2 No validation of legacy stage value (trusted as-is)
- [ ] 6.3 Tests: file present, absent, whitespace handling

## 7. Stage Derivation Algorithm

- [ ] 7.1 Rewrite refreshState(c *Change) error with new precedence:
  - [ ] 7.1a Derive progress from tasks
  - [ ] 7.1b If archived, set archived and return (no further rules)
  - [ ] 7.1c If .specsync.yaml stage exists, use it
  - [ ] 7.1d If .status exists, use it (with conflict warning if both exist)
  - [ ] 7.1e If tasks complete, set complete
  - [ ] 7.1f Default to active
- [ ] 7.2 Set StageSource on each path
- [ ] 7.3 Implement warnConflict() to stderr if .specsync.yaml and .status disagree
- [ ] 7.4 Unit tests: all six precedence paths

## 8. Archived Precedence Fix

- [ ] 8.1 Verify refreshState returns immediately for archived changes
- [ ] 8.2 Verify .specsync.yaml stage is ignored for archived changes
- [ ] 8.3 Verify .status file is ignored for archived changes
- [ ] 8.4 Tests: archived with .status, archived with .specsync.yaml, both present

## 9. Error Handling in LoadChange

- [ ] 9.1 LoadChange() now returns error if .specsync.yaml is invalid
- [ ] 9.2 Errors are descriptive (YAML parse, version, stage validation, priority validation)
- [ ] 9.3 Callers of LoadChange must handle errors appropriately:
  - [ ] 9.3a Main CLI: propagate (fail that change)
  - [ ] 9.3b Tests: verify error message content

## 10. Tests: Derivation Paths

- [ ] 10.1 Archived folder: stage=archived, stageSource=folder (ignore .status and .specsync.yaml)
- [ ] 10.2 .specsync.yaml stage present: use that, stageSource=metadata
- [ ] 10.3 .status present, no .specsync.yaml: use that, stageSource=legacy-status
- [ ] 10.4 Both files present, disagree: use .specsync.yaml, warn to stderr
- [ ] 10.5 Both files present, agree: use value, no warning
- [ ] 10.6 No explicit source, all tasks done: stage=complete, stageSource=tasks
- [ ] 10.7 No explicit source, some tasks done: stage=active, stageSource=default
- [ ] 10.8 No explicit source, no tasks: stage=active, stageSource=default

## 11. Tests: Custom Stages

- [ ] 11.1 Valid custom stage (qa-ready, needs-design, etc.): accepted, stored in Stage
- [ ] 11.2 Invalid custom stage (spaces, uppercase, Waiting!!!): error with pattern message
- [ ] 11.3 IsCanonicalStage returns false for custom stages
- [ ] 11.4 Custom stages work in both .specsync.yaml and .status

## 12. Tests: Priority

- [ ] 12.1 Valid priority 1: accepted
- [ ] 12.2 Valid priority 50: accepted
- [ ] 12.3 Valid priority 100: accepted
- [ ] 12.4 Priority 0: error
- [ ] 12.5 Priority 101: error
- [ ] 12.6 Priority -1: error
- [ ] 12.7 Priority absent in .specsync.yaml: Priority is nil
- [ ] 12.8 Priority: banana in .specsync.yaml: error on load

## 13. Tests: Metadata Parsing

- [ ] 13.1 Valid .specsync.yaml with version, stage, priority: all loaded
- [ ] 13.2 .specsync.yaml missing version field: treated as version 1
- [ ] 13.3 .specsync.yaml missing stage and priority: returns empty metadata
- [ ] 13.4 .specsync.yaml version: 2: error
- [ ] 13.5 Malformed YAML: error with parse detail
- [ ] 13.6 .specsync.yaml absent: no error, returns nil metadata

## 14. Tests: Task Progress

- [ ] 14.1 No tasks.md: progress=no-tasks
- [ ] 14.2 tasks.md empty: progress=no-tasks
- [ ] 14.3 tasks.md with 0/5 tasks checked: progress=not-started
- [ ] 14.4 tasks.md with 2/5 tasks checked: progress=in-progress
- [ ] 14.5 tasks.md with 5/5 tasks checked: progress=complete

## 15. Tests: OpenSpec Compatibility

- [ ] 15.1 OpenSpec list --json ignores .specsync.yaml
- [ ] 15.2 OpenSpec show includes task count, ignores stage/priority
- [ ] 15.3 Moving change to archive/ leaves .specsync.yaml behind (gitignored)

## 16. Documentation

- [ ] 16.1 Add godoc comments to TaskProgress enum
- [ ] 16.2 Add godoc comments to Stage enum (with canonical values listed)
- [ ] 16.3 Add godoc comments to StageSource enum
- [ ] 16.4 Add example in change.go comments showing .specsync.yaml format
- [ ] 16.5 Document stage derivation algorithm in change.go comments
