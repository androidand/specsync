# Tasks

## 1. specsync changes — Implementation

- [ ] 1.1 Add `changes` subcommand to cmd/specsync/main.go
- [ ] 1.2 Parse flags: -stage, -sort, -json, -openspec
- [ ] 1.3 Load all changes via LoadChanges()
- [ ] 1.4 Implement filter logic for -stage (comma-separated, case-sensitive)
- [ ] 1.5 Implement sort logic: canonical stage order (default), then priority, then slug
- [ ] 1.6 Implement alternate sort: -sort priority (priority within stage, then slug)

## 2. specsync changes — Table Output

- [ ] 2.1 Implement formatChangeTable(): render headers and rows
- [ ] 2.2 Columns: STAGE, PRIORITY, SLUG, PROGRESS, TASKS, TITLE
- [ ] 2.3 PRIORITY shows "-" for unset (not 0 or blank)
- [ ] 2.4 PROGRESS shows task-derived value (no-tasks, not-started, in-progress, complete)
- [ ] 2.5 TASKS shows "completed/total" format (e.g., "3/8")
- [ ] 2.6 TITLE truncates to ~60 chars if needed
- [ ] 2.7 Group by stage visually (blank line between stages, or per-stage headers)

## 3. specsync changes — JSON Output

- [ ] 3.1 Implement marshalChangeJSON(): convert Change to JSON object
- [ ] 3.2 Fields: slug, title, stage, canonicalStage, stageSource, priority, taskProgress, completedTasks, totalTasks, archived, diagnostics
- [ ] 3.3 priority is null (not 0) when nil
- [ ] 3.4 canonicalStage is boolean (true for backlog/blocked/active/in-review/complete/archived, false for custom)
- [ ] 3.5 diagnostics is array (empty if no issues)
- [ ] 3.6 Output entire array with proper JSON formatting

## 4. specsync changes — Diagnostics

- [ ] 4.1 Implement diagnostic struct: code, severity, message
- [ ] 4.2 Detect unmapped stage: custom stage with no board mapping (can add later)
- [ ] 4.3 Detect invalid stage: stage fails ValidateStage
- [ ] 4.4 Detect invalid priority: priority fails validation
- [ ] 4.5 Detect parse errors: malformed .specsync.yaml (but still report in output)
- [ ] 4.6 Warnings go to diagnostics array, not stderr (for JSON cleanliness)

## 5. specsync changes — Exit Code & Error Handling

- [ ] 5.1 Exit code 0 if any changes load successfully (even with diagnostics)
- [ ] 5.2 Exit code non-zero only if openspec/ directory missing or parse failure
- [ ] 5.3 Print error details to stderr for critical issues
- [ ] 5.4 Test: missing openspec/, malformed openspec changes

## 6. specsync set-stage — Validation

- [ ] 6.1 Add validateSlug(): reject empty, path traversal (.., /), uppercase/spaces
- [ ] 6.2 Validate against pattern ^[a-z0-9][a-z0-9_-]+$ (or similar convention)
- [ ] 6.3 Error messages suggest valid slug format
- [ ] 6.4 Test: various invalid slugs

## 7. specsync set-stage — Stage Argument

- [ ] 7.1 Parse <stage> argument; accept "auto" as special value
- [ ] 7.2 If stage != "auto", validate via ValidateStage()
- [ ] 7.3 Reject if stage fails validation (custom pattern check)
- [ ] 7.4 Test: canonical stages, custom stages, invalid stages, "auto"

## 8. specsync set-stage — Core Logic

- [ ] 8.1 Locate change directory (openspec/changes/<slug>/)
- [ ] 8.2 Check if archived; reject if yes
- [ ] 8.3 Load current .specsync.yaml (if exists)
- [ ] 8.4 Load legacy .status (if exists)
- [ ] 8.5 Load and validate current metadata (fail if malformed)
- [ ] 8.6 Mutate metadata:
  - [ ] 8.6a If stage == "auto": remove stage field, delete .status
  - [ ] 8.6b Else: set stage field, delete .status (migration)
- [ ] 8.7 If metadata now empty, delete .specsync.yaml entirely
- [ ] 8.8 Write atomically (temp file + rename)

## 9. specsync set-stage — Error Handling

- [ ] 9.1 Slug not found: error message "change not found: <slug>"
- [ ] 9.2 Archived change: error "cannot mutate archived change <slug>"
- [ ] 9.3 Invalid stage: error with pattern message
- [ ] 9.4 Malformed .specsync.yaml: error "fix .specsync.yaml before updating <slug>"
- [ ] 9.5 Write failures: propagate with context

## 10. specsync set-priority — Parsing & Validation

- [ ] 10.1 Parse <number> argument; accept "unset" as special value
- [ ] 10.2 If number != "unset", parse as integer
- [ ] 10.3 Validate 1 ≤ number ≤ 100; error if out of range
- [ ] 10.4 Error message: "priority must be between 1 and 100; got <value>"
- [ ] 10.5 Test: boundary values (0, 1, 100, 101), non-integer, "unset"

## 11. specsync set-priority — Core Logic

- [ ] 11.1 Locate change directory
- [ ] 11.2 Load current .specsync.yaml (if exists)
- [ ] 11.3 Load and validate current metadata (fail if malformed)
- [ ] 11.4 Mutate metadata:
  - [ ] 11.4a If number == "unset": remove priority field
  - [ ] 11.4b Else: set priority field to number
- [ ] 11.5 If metadata now empty, delete .specsync.yaml entirely
- [ ] 11.6 Write atomically

## 12. specsync set-priority — Archived Behavior

- [ ] 12.1 Allow set-priority on archived changes (priority can be set even if not active)
- [ ] 12.2 Useful for prioritizing work if archived change is re-activated later

## 13. Atomic Write Implementation

- [ ] 13.1 Implement atomicWrite(path, data, perm): write to temp, rename
- [ ] 13.2 Temp file: <path>.tmp in same directory
- [ ] 13.3 Delete temp on rename failure
- [ ] 13.4 Clean up temp on Ctrl-C or other interruption (best-effort)

## 14. Help & Usage

- [ ] 14.1 `specsync changes --help` displays usage, flags, examples
- [ ] 14.2 `specsync set-stage --help` displays usage, examples
- [ ] 14.3 `specsync set-priority --help` displays usage, examples
- [ ] 14.4 Missing required arguments trigger help + error

## 15. Tests: specsync changes

- [ ] 15.1 List all changes, grouped by stage
- [ ] 15.2 Filter by single stage
- [ ] 15.3 Filter by multiple stages
- [ ] 15.4 Sort by priority (default stage order + priority)
- [ ] 15.5 JSON output format
- [ ] 15.6 JSON includes diagnostics
- [ ] 15.7 Priority null in JSON, not 0
- [ ] 15.8 Missing openspec/ directory

## 16. Tests: specsync set-stage

- [ ] 16.1 Create .specsync.yaml with stage
- [ ] 16.2 Migrate .status → .specsync.yaml, delete .status
- [ ] 16.3 Preserve priority when changing stage
- [ ] 16.4 set-stage auto removes stage, preserves priority
- [ ] 16.5 set-stage auto deletes empty .specsync.yaml
- [ ] 16.6 Archived changes reject mutation
- [ ] 16.7 Invalid stage rejected
- [ ] 16.8 Malformed .specsync.yaml blocks mutation
- [ ] 16.9 Slug not found
- [ ] 16.10 Path traversal rejected
- [ ] 16.11 Atomic write: no partial files on error

## 17. Tests: specsync set-priority

- [ ] 17.1 Create .specsync.yaml with priority
- [ ] 17.2 Preserve stage when changing priority
- [ ] 17.3 set-priority unset removes priority
- [ ] 17.4 set-priority unset deletes empty .specsync.yaml
- [ ] 17.5 Out-of-range (0, 101) rejected
- [ ] 17.6 Boundary values (1, 100) accepted
- [ ] 17.7 Archived changes accept priority
- [ ] 17.8 Malformed .specsync.yaml blocks mutation
- [ ] 17.9 Slug not found
- [ ] 17.10 Atomic write: no partial files on error

## 18. Integration Tests

- [ ] 18.1 Create change, set stage, verify specsync changes output
- [ ] 18.2 Set priority, verify JSON includes value
- [ ] 18.3 set-stage auto, verify task-derived stage is restored
- [ ] 18.4 Multiple mutations in sequence, verify state consistency
- [ ] 18.5 Test against this repo's own changes

## 19. Documentation

- [ ] 19.1 Update SKILL.md with changes, set-stage, set-priority docs
- [ ] 19.2 Include example usage for each command
- [ ] 19.3 Document JSON output format in README or separate doc
- [ ] 19.4 Document slug validation rules
- [ ] 19.5 Example: list backlog by priority, then prioritize
