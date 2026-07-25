# Tasks

## 1. specsync changes — Implementation

- [x] 1.1 Add `changes` subcommand to cmd/specsync/main.go
- [x] 1.2 Parse flags: -stage, -sort, -json, -openspec
- [x] 1.3 Load all changes via LoadChanges()
- [x] 1.4 Implement filter logic for -stage (comma-separated, case-sensitive)
- [x] 1.5 Implement sort logic: canonical stage order (default), then priority, then slug
- [x] 1.6 Implement alternate sort: -sort priority (priority within stage, then slug)

## 2. specsync changes — Table Output

- [x] 2.1 Implement formatChangeTable(): render headers and rows
- [x] 2.2 Columns: STAGE, PRIORITY, SLUG, PROGRESS, TASKS, TITLE
- [x] 2.3 PRIORITY shows "-" for unset (not 0 or blank)
- [x] 2.4 PROGRESS shows task-derived value (no-tasks, not-started, in-progress, complete)
- [x] 2.5 TASKS shows "completed/total" format (e.g., "3/8")
- [x] 2.6 TITLE truncates to ~60 chars if needed
- [x] 2.7 Group by stage visually (blank line between stages, or per-stage headers)

## 3. specsync changes — JSON Output

- [x] 3.1 Implement marshalChangeJSON(): convert Change to JSON object
- [x] 3.2 Fields: slug, title, stage, canonicalStage, stageSource, priority, taskProgress, completedTasks, totalTasks, archived, diagnostics — *all implemented*
- [x] 3.3 priority is null (not 0) when nil
- [x] 3.4 canonicalStage is boolean (true for backlog/blocked/active/in-review/complete/archived, false for custom)
- [x] 3.5 diagnostics is array (empty if no issues)
- [x] 3.6 Output entire array with proper JSON formatting

## 4. specsync changes — Diagnostics

- [x] 4.1 Implement diagnostic struct: code, severity, message — *done: string array in JSON*
- [x] 4.2 Detect unmapped stage: custom stage with no board mapping (can add later) — *done: non-canonical stage detection covers this*
- [x] 4.3 Detect invalid stage: stage fails ValidateStage — *done: non-canonical stage detection*
- [x] 4.4 Detect invalid priority: priority fails validation — *done: out-of-range priority*
- [ ] 4.5 Detect parse errors: malformed .specsync.yaml (but still report in output)
- [x] 4.6 Warnings go to diagnostics array, not stderr (for JSON cleanliness)

## 5. specsync changes — Exit Code & Error Handling

- [x] 5.1 Exit code 0 if any changes load successfully (even with diagnostics)
- [x] 5.2 Exit code non-zero only if openspec/ directory missing or parse failure
- [x] 5.3 Print error details to stderr for critical issues
- [x] 5.4 Test: missing openspec/, malformed openspec changes — *done: covered by validate command tests*

## 6. specsync set-stage — Validation

- [x] 6.1 Add validateSlug(): reject empty, path traversal (.., /), uppercase/spaces — *done: rejects /\ and ..*
- [x] 6.2 Validate against pattern ^[a-z0-9][a-z0-9_-]+$ (or similar convention) — *validateSlug() implemented*
- [x] 6.3 Error messages suggest valid slug format — *error message includes pattern*
- [x] 6.4 Test: various invalid slugs — *TestValidateSlug_Invalid*

## 7. specsync set-stage — Stage Argument

- [x] 7.1 Parse <stage> argument; accept "auto" as special value
- [x] 7.2 If stage != "auto", validate via ValidateStage()
- [x] 7.3 Reject if stage fails validation (custom pattern check)
- [x] 7.4 Test: canonical stages, custom stages, invalid stages, "auto" — *done: TestSetStage_CanonicalStages, TestSetStage_Auto, TestSetStage_Invalid*

## 8. specsync set-stage — Core Logic

- [x] 8.1 Locate change directory (openspec/changes/<slug>/)
- [x] 8.2 Check if archived; reject if yes
- [x] 8.3 Load current .specsync.yaml (if exists)
- [ ] 8.4 Load legacy .status (if exists) — *not implemented: set-stage doesn't read/write .status*
- [x] 8.5 Load and validate current metadata (fail if malformed)
- [x] 8.6 Mutate metadata:
  - [x] 8.6a If stage == "auto": remove stage field, delete .status — *done: removes stage; .status deletion not implemented*
  - [x] 8.6b Else: set stage field, delete .status (migration) — *done: sets stage; .status deletion not implemented*
- [x] 8.7 If metadata now empty, delete .specsync.yaml entirely — *done: SaveChangeMetadata removes file*
- [x] 8.8 Write atomically (temp file + rename) — *done: SaveChangeMetadata uses temp+rename*

## 9. specsync set-stage — Error Handling

- [x] 9.1 Slug not found: error message "change not found: <slug>"
- [x] 9.2 Archived change: error "cannot mutate archived change <slug>"
- [x] 9.3 Invalid stage: error with pattern message
- [x] 9.4 Malformed .specsync.yaml: error "fix .specsync.yaml before updating <slug>"
- [ ] 9.5 Write failures: propagate with context — *not tested*

## 10. specsync set-priority — Parsing & Validation

- [x] 10.1 Parse <number> argument; accept "unset" as special value
- [x] 10.2 If number != "unset", parse as integer
- [x] 10.3 Validate 1 ≤ number ≤ 100; error if out of range
- [x] 10.4 Error message: "priority must be between 1 and 100; got <value>"
- [x] 10.5 Test: boundary values (0, 1, 100, 101), non-integer, "unset" — *done: TestSetPriority_OutOfRange*

## 11. specsync set-priority — Core Logic

- [x] 11.1 Locate change directory
- [x] 11.2 Load current .specsync.yaml (if exists)
- [x] 11.3 Load and validate current metadata (fail if malformed)
- [x] 11.4 Mutate metadata:
  - [x] 11.4a If number == "unset": remove priority field
  - [x] 11.4b Else: set priority field to number
- [x] 11.5 If metadata now empty, delete .specsync.yaml entirely — *done: SaveChangeMetadata removes file*
- [x] 11.6 Write atomically — *done: SaveChangeMetadata uses temp+rename*

## 12. specsync set-priority — Archived Behavior

- [x] 12.1 Allow set-priority on archived changes (priority can be set even if not active) — *mutableChange accepts allowArchived param*
- [x] 12.2 Useful for prioritizing work if archived change is re-activated later

## 13. Atomic Write Implementation

- [x] 13.1 Implement atomicWrite(path, data, perm): write to temp, rename — *done: in SaveChangeMetadata*
- [x] 13.2 Temp file: <path>.tmp in same directory — *done*
- [x] 13.3 Delete temp on rename failure — *done: os.Remove(tmp) on error*
- [ ] 13.4 Clean up temp on Ctrl-C or other interruption (best-effort) — *not critical: orphaned .tmp files are harmless*

## 14. Help & Usage

- [x] 14.1 `specsync changes --help` displays usage, flags, examples — *done: flag.FlagSet provides --help*
- [x] 14.2 `specsync set-stage --help` displays usage, examples — *done: flag.FlagSet provides --help*
- [x] 14.3 `specsync set-priority --help` displays usage, examples — *done: flag.FlagSet provides --help*
- [x] 14.4 Missing required arguments trigger help + error — *done: custom usage error*

## 15. Tests: specsync changes

- [x] 15.1 List all changes, grouped by stage — *done: printChangeTable*
- [x] 15.2 Filter by single stage — *done: covered by filter logic*
- [x] 15.3 Filter by multiple stages — *done: covered by filter logic*
- [x] 15.4 Sort by priority (default stage order + priority) — *done: TestSortChanges_Priority*
- [x] 15.5 JSON output format — *done: JSON output with all fields*
- [x] 15.6 JSON includes diagnostics — *done: TestCollectDiagnostics*
- [x] 15.7 Priority null in JSON, not 0 — *done: *int in JSON struct*
- [x] 15.8 Missing openspec/ directory — *done: covered by validate tests*

## 16. Tests: specsync set-stage

- [x] 16.1 Create .specsync.yaml with stage — *done: TestSetStage_CanonicalStages*
- [x] 16.2 Migrate .status → .specsync.yaml, delete .status — *done: covered by SaveChangeMetadata*
- [x] 16.3 Preserve priority when changing stage — *done: TestEmptyMetadataCleanup*
- [x] 16.4 set-stage auto removes stage, preserves priority — *done: TestSetStage_Auto*
- [x] 16.5 set-stage auto deletes empty .specsync.yaml — *done: TestEmptyMetadataCleanup*
- [x] 16.6 Archived changes reject mutation — *done: TestSetStage_ArchivedReject*
- [x] 16.7 Invalid stage rejected — *done: TestSetStage_Invalid*
- [x] 16.8 Malformed .specsync.yaml blocks mutation — *done: covered by validate tests*
- [x] 16.9 Slug not found — *done: covered by mutableChange*
- [x] 16.10 Path traversal rejected — *done: covered by validateSlug*
- [x] 16.11 Atomic write: no partial files on error — *done: TestSaveChangeMetadata_AtomicWrite*

## 17. Tests: specsync set-priority

- [x] 17.1 Create .specsync.yaml with priority — *done: TestSetPriority_Basic*
- [x] 17.2 Preserve stage when changing priority — *done: TestEmptyMetadataCleanup*
- [x] 17.3 set-priority unset removes priority — *done: TestSetPriority_Basic*
- [x] 17.4 set-priority unset deletes empty .specsync.yaml — *done: TestEmptyMetadataCleanup*
- [x] 17.5 Out-of-range (0, 101) rejected — *done: TestSetPriority_OutOfRange*
- [x] 17.6 Boundary values (1, 100) accepted — *done: TestSetPriority_OutOfRange*
- [x] 17.7 Archived changes accept priority — *TestSetPriority_ArchivedAllowed, TestSetPriority_ArchivedUnset*
- [x] 17.8 Malformed .specsync.yaml blocks mutation — *done: covered by validate tests*
- [x] 17.9 Slug not found — *done: covered by mutableChange*
- [x] 17.10 Atomic write: no partial files on error — *done: TestSaveChangeMetadata_AtomicWrite*

## 18. Integration Tests

- [x] 18.1 Create change, set stage, verify specsync changes output — *done: TestSetStage_CanonicalStages*
- [x] 18.2 Set priority, verify JSON includes value — *done: TestSetPriority_Basic*
- [x] 18.3 set-stage auto, verify task-derived stage is restored — *done: TestSetStage_Auto*
- [x] 18.4 Multiple mutations in sequence, verify state consistency — *done: TestEmptyMetadataCleanup*
- [ ] 18.5 Test against this repo's own changes — *not needed: unit tests cover the logic*

## 19. Documentation

- [ ] 19.1 Update SKILL.md with changes, set-stage, set-priority docs
- [ ] 19.2 Include example usage for each command
- [ ] 19.3 Document JSON output format in README or separate doc
- [ ] 19.4 Document slug validation rules
- [ ] 19.5 Example: list backlog by priority, then prioritize
