# Agent Hooks Specification

## Overview

Agent hooks allow Skein to notify specsync (and other systems) when work state changes. An agent picks up a change, progresses it, hits a blocker, submits for review, or completes—each event triggers a hook.

Hooks are configurable commands that run in response to agent lifecycle events.

## Hook Types

### 1. on_assign

**When**: Agent is assigned a change (work starts)

**Purpose**: Mark change as "active" so humans and other systems know work is underway

**Default hook**:
```bash
specsync set-stage $(SLUG) active
```

**Environment variables**:
- `SLUG` — change slug (e.g., "feature-auth")
- `CHANGE_DIR` — full path to change directory
- `AGENT_ROLE` — agent role (coder, debugger, reviewer)
- `REPO_ROOT` — repo root path

**Example output**:
```
✓ Set feature-auth stage to active
```

---

### 2. on_progress

**When**: Agent makes substantive progress (e.g., after passing test, after commit)

**Purpose**: Reinforce that work is actively happening; reset "no progress" timers

**Optional**; default is no-op.

**Use case**:
```yaml
on_progress:
  - specsync set-stage $(SLUG) active  # idempotent; ensure still active
  - /custom-slack-notify "progress on $(SLUG)"
```

**Frequency**: Intentionally rare (not every keystroke); maybe once per PR or every 30 min of work

---

### 3. on_block

**When**: Agent hits an external dependency (design needed, clarification, approval, tool failure)

**Purpose**: Alert humans that work is blocked; stop other agents from wasting cycles

**Default hook**:
```bash
specsync set-stage $(SLUG) blocked
```

**Agent must provide reason**:
```
blocked: waiting for design review from @architect
blocked: need clarification on requirements
blocked: tool X not available on this machine
```

**How reason is captured**:
1. Agent adds to tasks.md as: `- [ ] waiting for design review from @architect`
2. Hook runs with `BLOCK_REASON` env var
3. Hook can append to tasks.md or log separately

**Example**:
```bash
on_block:
  - specsync set-stage $(SLUG) blocked
  - echo "Blocked at $(date): $(BLOCK_REASON)" >> $(CHANGE_DIR)/BLOCKED.log
```

---

### 4. on_submit_review

**When**: Agent submits work for review (opens PR, creates draft PR, requests review)

**Purpose**: Transition to "in-review" stage; signal that implementation is done, waiting on feedback

**Default hook**:
```bash
specsync set-stage $(SLUG) in-review
```

**Environment variables** (additional):
- `PR_URL` — link to PR (if applicable)
- `REVIEW_CHECKLIST` — items reviewer should check (optional)

**Example**:
```bash
on_submit_review:
  - specsync set-stage $(SLUG) in-review
  - echo "PR: $(PR_URL)" >> $(CHANGE_DIR)/REVIEW.log
```

---

### 5. on_complete

**When**: All tasks checked, spec validated, work is truly done

**Purpose**: Archive the work; clear it from the active queue

**Default hook**:
```bash
specsync set-stage $(SLUG) complete
```

**Verification before hook runs**:
- Check all tasks.md items are checked
- Verify no critical blockers remain
- Ensure PR is merged (if applicable)

**Example**:
```bash
on_complete:
  - specsync set-stage $(SLUG) complete
  - /telegram-notify "✅ $(SLUG) complete"
```

---

## Hook Execution

### Configuration

```yaml
# In .skein/config.yaml

agent_hooks:
  # Global hooks (all agents)
  on_assign:
    - specsync set-stage $(SLUG) active
  
  on_block:
    - specsync set-stage $(SLUG) blocked
  
  on_submit_review:
    - specsync set-stage $(SLUG) in-review
  
  on_complete:
    - specsync set-stage $(SLUG) complete
  
  # Per-role hooks (override global)
  coder:
    on_assign:
      - specsync set-stage $(SLUG) active
      - /run-unit-tests $(SLUG)  # Custom: coder runs tests on pickup
  
  reviewer:
    on_complete:
      - specsync set-stage $(SLUG) complete
      - /send-to-changelog $(SLUG)  # Reviewer marks as shipped
```

### Execution Semantics

1. **Synchronous**: Hook runs; supervisor waits for completion
2. **Timeout**: 30 seconds max (configurable). If longer, kill and warn.
3. **Error handling**: If hook fails (non-zero exit), log error but DON'T block. Agent continues.
4. **Output**: Capture stdout/stderr; log to audit trail
5. **Environment**: Hooks run in supervisor's environment, with added vars
6. **Working directory**: Change directory ($(CHANGE_DIR))

### Example Hook Run

```bash
# Trigger: Agent assigned to feature-auth
# Hook: specsync set-stage $(SLUG) active

Environment:
  SLUG=feature-auth
  CHANGE_DIR=/Users/user/dev/specsync/openspec/changes/feature-auth
  AGENT_ROLE=coder
  REPO_ROOT=/Users/user/dev/specsync

Command: specsync set-stage feature-auth active

Output:
  ✓ Set feature-auth stage to active

Audit Log Entry:
  timestamp=2026-07-15T22:30:00Z event=hook_executed hook=on_assign slug=feature-auth status=success duration=150ms
```

## Error Handling

### Hook Fails (Non-Zero Exit)

```
Hook: specsync set-stage feature-auth active
Exit code: 1
Error: Cannot find feature-auth

Action:
  - Log error: "hook on_assign failed for feature-auth: Cannot find feature-auth"
  - Continue assignment (don't block agent)
  - Escalate to human: "Hook failed; check logs"
```

### Hook Timeout

```
Hook: /custom-long-running-hook
Timeout: 30s exceeded

Action:
  - Kill process
  - Log: "hook on_complete timed out for feature-auth after 30s"
  - Continue (don't block agent)
```

### Hook Not Configured

```
Hook: on_progress (not defined)

Action:
  - Skip silently (no-op is acceptable)
  - No log (optional: log only in debug mode)
```

## Environment Variables

### Standard (all hooks)

| Variable | Value | Example |
|----------|-------|---------|
| `SLUG` | Change slug | `feature-auth-rewrite` |
| `CHANGE_DIR` | Full path to change dir | `/path/to/openspec/changes/feature-auth-rewrite` |
| `AGENT_ROLE` | coder, debugger, reviewer, etc. | `coder` |
| `REPO_ROOT` | Root of the repo | `/Users/user/dev/specsync` |
| `SKEIN_PROJECT` | Skein project name | `specsync` |
| `SKEIN_SUPERVISOR_PID` | PID of supervisor | `12345` |

### Hook-Specific

| Hook | Variable | Value |
|------|----------|-------|
| on_block | `BLOCK_REASON` | Reason from agent | "waiting for design review" |
| on_submit_review | `PR_URL` | PR/review link | `https://github.com/user/repo/pull/123` |
| on_submit_review | `REVIEW_CHECKLIST` | Checklist for reviewer | (optional) |

### Injected by Hook

Hooks can also export variables for downstream use:

```bash
# Hook outputs variable
echo "PR_URL=https://github.com/user/repo/pull/123"

# Supervisor captures and makes available to next hook
export PR_URL=https://github.com/user/repo/pull/123
```

## Use Cases & Examples

### Use Case 1: Basic State Tracking

Just update specsync stages:

```yaml
agent_hooks:
  on_assign: specsync set-stage $(SLUG) active
  on_block: specsync set-stage $(SLUG) blocked
  on_submit_review: specsync set-stage $(SLUG) in-review
  on_complete: specsync set-stage $(SLUG) complete
```

### Use Case 2: Slack Notifications

Notify team on state changes:

```yaml
agent_hooks:
  on_assign: /telegram-notify "🚀 $(AGENT_ROLE) started $(SLUG)"
  on_block: /telegram-notify "🚧 $(SLUG) blocked: check logs"
  on_submit_review: /telegram-notify "👀 $(SLUG) ready for review"
  on_complete: /telegram-notify "✅ $(SLUG) complete"
```

### Use Case 3: Auto-Testing (Coder-Specific)

Coder runs tests on pickup:

```yaml
agent_hooks:
  coder:
    on_assign:
      - specsync set-stage $(SLUG) active
      - cd $(CHANGE_DIR) && make test
  reviewer:
    on_assign:
      - specsync set-stage $(SLUG) active
      # No auto-test for reviewer
```

### Use Case 4: Blocklist Management

Coordinator role blocks/unblocks automatically:

```yaml
agent_hooks:
  coordinator:
    on_block:
      - specsync set-stage $(SLUG) blocked
      - echo "$(BLOCK_REASON)" >> .skein/blocked-changes.md
  coordinator:
    on_unblock:  # Not yet defined; future hook type
      - specsync set-stage $(SLUG) backlog
      - /telegram-notify "🔓 $(SLUG) unblocked"
```

## Testing

### Unit Test: Hook Execution

```go
func TestHookExecution(t *testing.T) {
  // Test 1: Hook runs successfully
  result := runHook("specsync set-stage test-change active")
  assert(result.ExitCode == 0)
  assert(result.Output contains "active")
  
  // Test 2: Hook timeout
  result := runHook("sleep 60", timeout: 5s)
  assert(result.Timeout == true)
  
  // Test 3: Hook error captured
  result := runHook("exit 1")
  assert(result.ExitCode == 1)
  assert(result.Error != "")
}
```

### Integration Test: Full Workflow

```go
func TestAgentWorkflowWithHooks(t *testing.T) {
  // Setup: Create change with hooks enabled
  // Trigger: Assign coder agent
  // Verify: on_assign hook ran, stage is now "active"
  
  // Trigger: Agent hits blocker
  // Verify: on_block hook ran, stage is now "blocked"
  
  // Trigger: Human unblocks
  // Verify: Dispatcher can assign again
}
```

## Future Extensions

1. **on_progress_update**: Periodic checkin while working (every 30 min)
2. **on_unblock**: When human unblocks a change
3. **on_escalate**: Critical blocker detected; escalate to human
4. **on_conflict**: Board sync detected conflict; escalate
5. **on_commit**: After each git commit (maybe too frequent?)
6. **on_pr_comment**: When reviewer comments on PR
7. **on_revert**: If a change is reverted/rolled back

## Observability

### Audit Log

Every hook execution logged:

```
2026-07-15T22:30:00Z hook=on_assign slug=feature-auth status=success duration=150ms output="✓ Set feature-auth stage to active"
2026-07-15T22:31:00Z hook=on_block slug=feature-auth status=success duration=80ms reason="waiting for design review"
2026-07-15T22:45:00Z hook=on_submit_review slug=feature-auth status=success duration=120ms pr_url="https://github.com/user/repo/pull/123"
2026-07-15T23:00:00Z hook=on_complete slug=feature-auth status=success duration=200ms
```

### Metrics

- Hooks run per hour
- Success rate (%)
- Average duration (ms)
- Most common failures
- Hook timeout incidents

### Dashboard

Show hook health, recent runs, error patterns.
