# Error Messages & User Guidance

## Design Principles

Error messages in specsync should:

1. **State the problem clearly** — What went wrong?
2. **Explain why** — Why is this invalid or problematic?
3. **Suggest a fix** — What can the user do about it?
4. **Be concise** — < 2 sentences in the error, with optional details

---

## Current Error Messages (Audit)

### ✅ Good Examples

```bash
# set-stage with invalid slug
$ specsync set-stage "../../etc/passwd" active
error: invalid slug: ../../etc/passwd

# set-stage on archived change
$ specsync set-stage archived-change active
error: cannot mutate archived change archived-change

# set-priority with invalid range
$ specsync set-priority my-change 101
error: priority must be between 1 and 100; got 101

# change not found
$ specsync set-stage nonexistent active
error: change not found: nonexistent

# invalid stage value
$ specsync set-stage my-change invalid-stage
error: invalid stage "invalid-stage"; must be canonical or match ^[a-z0-9][a-z0-9-]{0,63}$
```

### ⚠️ Messages That Could Be Improved

```bash
# Too vague
$ specsync sync --project invalid
error: invalid board target

# Better version would be:
error: invalid board target "invalid"; expected format "owner/number" (e.g., "myorg/5")

# Missing context
$ specsync pull --issue 42
error: provider github cannot read issues

# Better version would be:
error: provider "github" does not support issue reading; make sure gh is installed and authenticated

# Unclear recovery
$ specsync sync --status-map "bad"
error: parse status map: invalid format

# Better version would be:
error: status map invalid format: "bad"; expected "stage=Name,..." (e.g., "active=In Progress,complete=Done")
```

---

## Error Message Templates

### When a change is not found

```
error: change not found: <slug>
  • Did you mean: <similar-slug> ?
  • Check: ls openspec/changes/ | grep <partial-slug>
```

### When a stage is invalid

```
error: invalid stage "<value>"
  • Canonical stages: backlog, active, blocked, in-review, complete, archived
  • Custom stages must match: ^[a-z0-9][a-z0-9-]{0,63}$
  • Example: active, in-review, awaiting-review
```

### When priority is out of range

```
error: priority must be 1–100; got <value>
  • 1-29   = VERY_LOW (polish, docs, cleanup)
  • 30-49  = LOW (nice-to-have, refactoring)
  • 50-69  = NORMAL (regular work)
  • 70-89  = HIGH (user-facing features)
  • 90-98  = CRITICAL (security, data loss prevention)
  • 99     = FOCUS (human explicitly said "work on this")
```

### When a change is archived and you try to mutate it

```
error: cannot mutate archived change "<slug>"
  • Archived changes are immutable to prevent accidental updates
  • To un-archive: mv openspec/changes/archive/<slug>/ openspec/changes/<slug>/
  • Then: specsync set-stage <slug> backlog
```

### When board configuration is invalid

```
error: invalid board target "<target>"; expected "owner/number"
  • Valid format: "myorg/5" or "my-org/123"
  • Find your board number: gh repo view --json projectsV2
  • Or set env: export SPECSYNC_PROJECT="myorg/5"
```

### When a provider is not available

```
error: provider "github" not configured
  • Install gh: https://cli.github.com/
  • Authenticate: gh auth login
  • Or set: export GH_TOKEN="github_pat_..."
```

### When tasks.md is malformed

```
error: invalid tasks.md for "<slug>"
  • Expected format: - [ ] task description or - [x] done task
  • Found unrecognized line: <problematic-line>
  • Example valid format:
    - [ ] Implement feature
    - [ ] Write tests
    - [x] Code review passed
```

### When metadata.json is corrupt

```
error: invalid .specsync/metadata.json for "<slug>"
  • Expected JSON with: { "version": 1, "stage": "...", "priority": ... }
  • Parse error: <json-error>
  • Fix: Delete the file to reset, or manually repair JSON
```

---

## Validation Improvements Needed

### Priority validation

Currently:
```go
if err != nil || priority < 1 || priority > 100 {
	fail(fmt.Errorf("priority must be between 1 and 100; got %s", priorityArg))
}
```

Should be enhanced to:
```go
priority, err := strconv.Atoi(priorityArg)
if err != nil {
	fail(fmt.Errorf("priority must be a number (1-100); got %q: %v", priorityArg, err))
}
if priority < 1 || priority > 100 {
	fail(fmt.Errorf(
		"priority must be between 1 and 100; got %d\n"+
		"  1-29   VERY_LOW (docs, cleanup)\n"+
		"  30-49  LOW (nice-to-have)\n"+
		"  50-69  NORMAL (regular work)\n"+
		"  70-89  HIGH (user-facing features)\n"+
		"  90-98  CRITICAL (security, data loss)\n"+
		"  99     FOCUS (human priority)",
		priority))
}
```

### Stage validation

Currently:
```go
if err := specsync.ValidateStage(specsync.Stage(stage)); err != nil {
	fail(err)
}
```

Output is good. Keep as-is.

### Slug validation

Currently:
```go
if strings.ContainsAny(slug, "/..") {
	fail(fmt.Errorf("invalid slug: %s", slug))
}
```

Should be:
```go
if strings.ContainsAny(slug, "/..") || strings.Contains(slug, "\\") {
	fail(fmt.Errorf(
		"invalid slug %q: contains path separators\n"+
		"  Slugs must be safe directory names: letters, numbers, hyphens\n"+
		"  Example valid slugs: feature-auth, bugfix-123, v2-refactor",
		slug))
}
```

### Board target validation

Should check format early:
```go
func validateBoardTarget(target string) error {
	parts := strings.Split(target, "/")
	if len(parts) != 2 {
		return fmt.Errorf(
			"board target %q invalid: expected 'owner/number' format\n"+
			"  Examples: myorg/5, my-company/123\n"+
			"  Find your board: gh repo view --json projectsV2",
			target)
	}
	// ... validate owner and number formats
}
```

---

## Error Handling Patterns

### Three-way merge conflicts

When local and remote both changed:
```
error: board state conflict for "<slug>"
  • Local stage: <local-stage>
  • Remote status: <remote-status>
  • Last synced: <time-ago>
  • Action: Manual review needed; re-run sync after deciding what to do
  • Log: See .specsync/board.json for full binding state
```

### Human board moves detected

When human moved card on board:
```
info: board: <slug> status update skipped
  • Reason: Human moved the card on the board
  • Local stage: <local-stage>
  • Remote status: <remote-status> (human set this)
  • Action: Status is correct; no sync needed (human's move respected)
```

### Dry-run vs real-run differences

```
DRY RUN — no github calls are made
  • Stage changes would be: active → in-review
  • Board would show: In Progress (not updated on dry-run)
  • To apply: Run without --dry-run flag
```

---

## Error Message Review Checklist

- [ ] **Problem statement**: User knows what went wrong
- [ ] **Root cause**: User understands why it failed
- [ ] **Recovery path**: User knows how to fix it
- [ ] **Concise**: < 3 lines before optional details
- [ ] **Actionable**: User can take immediate action
- [ ] **Examples**: When format is unclear, show example
- [ ] **No jargon**: Avoid technical terms without explanation
- [ ] **Positive tone**: State what CAN be done, not just what can't

---

## Testing Error Messages

Add test cases for error conditions:

```go
func TestSetPriorityErrors(t *testing.T) {
	tests := []struct {
		name      string
		slug      string
		priority  string
		wantError string
	}{
		{
			name:      "priority out of range high",
			priority:  "101",
			wantError: "must be between 1 and 100",
		},
		{
			name:      "priority not a number",
			priority:  "abc",
			wantError: "must be a number",
		},
		{
			name:      "invalid slug with path traversal",
			slug:      "../../etc/passwd",
			wantError: "invalid slug",
		},
	}
	// ... run tests and verify error messages
}
```

---

## Messages for Silent Failures

Some operations should fail loudly:

```bash
# BAD: silently ignores error
$ specsync pull --issue 42 --dry-run
# nothing happens, no error

# GOOD: reports what happened
$ specsync pull --issue 42 --dry-run
DRY RUN — would write openspec/changes/feature-123
  proposal.md
  tasks.md
  (use without --dry-run to apply)
```

---

## Consistency Across Commands

All commands should use consistent patterns:

| Situation | Pattern |
|-----------|---------|
| Required arg missing | `error: <command> requires <arg>`  |
| Invalid value | `error: <arg> invalid: <reason>` |
| File not found | `error: <file> not found` |
| Forbidden operation | `error: cannot <action> on <item>: <reason>` |
| Dry-run output | `DRY RUN — <what would happen>` |
| Success (quiet) | No output unless `--verbose` |

---

## Future: Structured error codes

Consider adding error codes for automation:

```go
const (
	ErrChangeNotFound     = "E001"
	ErrInvalidStage       = "E002"
	ErrPriorityOutOfRange = "E003"
	ErrArchivedImmutable  = "E004"
	ErrBoardConflict      = "E005"
)

// Output format
// ERROR[E001] change not found: my-change
```

This allows scripts to:
```bash
specsync set-stage foo bar 2>&1 | grep -q "E001" && echo "Not found"
```

---

## Summary

Error messages should guide users toward solutions, not just report problems.
Invest in clarity at the point of failure.
