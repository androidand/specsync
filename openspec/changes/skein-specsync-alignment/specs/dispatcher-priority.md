# Dispatcher Priority Specification

## Overview

The dispatcher's job is to pick the next change to assign to an agent. Currently, it's random or FIFO. We need to make it **priority-aware** and **stage-aware**.

## Data Structures

### Change with Metadata

```go
type Change struct {
  Slug      string
  Dir       string
  Title     string
  Created   time.Time
  
  // Loaded from .specsync/metadata.json
  Metadata struct {
    Version  int    `json:"version"`
    Stage    string `json:"stage"`      // backlog, active, blocked, in-review, complete, archived
    Priority *int   `json:"priority"`   // 1-100, nil = default (0)
  }
  
  // Derived from tasks.md or other sources
  TaskCount     int
  TasksChecked  int
  DerivedStage  string  // from task completion
}
```

### Dispatch Decision

```go
type DispatchDecision struct {
  Slug        string
  Reason      string        // "priority=95", "oldest with priority=50", "no other available"
  Priority    int
  Stage       string
  CanAssign   bool
  SkipReason  string        // if CanAssign is false
}
```

## Priority Interpretation

### Values

- **1-100**: Explicit priority (higher = more urgent)
- **0 (or nil)**: Default (use creation date for tie-breaking)
- **Negative**: Reserved for future use

### Priority Tiers

```
99:  FOCUS — Human explicitly said "work on this"
90-98: CRITICAL — Security, data loss prevention, major outage
70-89: HIGH — User-facing features, important bugs
50-69: NORMAL — Regular work, standard features
30-49: LOW — Nice-to-have, refactoring, tech debt
1-29: VERY_LOW — Polish, docs, cleanup
0:    DEFAULT — No explicit priority; tie-break by age
```

### Derived Priority

If a change has no explicit priority, derive from:

```
derived_priority = base_priority_from_source

if source == "task_count":
  // More tasks = more complex = higher priority? Or lower (long tail)?
  // Decision: DEFER this for now, use default (0) always
  
if source == "has_ref":
  // Change already has a synced GitHub issue = some engagement
  // Hint: slightly higher (like +5), but not a hard rule
  // Decision: DEFER for now
  
if source == "committed":
  // Change has commits = active work
  // Hint: move to "active" stage instead of boosting priority
  // Decision: Use stage, not priority
```

**Decision**: Start simple. Priority is always explicit (set by human or default to 0). Derive stage from tasks and folder, but NOT from priority.

## Filtering Rules

### Assignable Stages

```go
func isAssignable(c Change) bool {
  switch c.Metadata.Stage {
  case "backlog":
    return true           // Ready to pick
  case "active":
    return true           // Already assigned; allow re-assignment if dropped
  case "blocked":
    return false          // Waiting on external; skip
  case "in-review":
    return false          // Under review; don't re-assign
  case "complete":
    return false          // Done; don't re-assign
  case "archived":
    return false          // Archived; skip
  case "":
    return true           // No stage set; treat as backlog
  default:
    return false          // Unknown stage; skip
  }
}
```

### Fallback Logic

If no "backlog" changes available:
1. Try "active" (pick up dropped work)
2. Try empty stage (unclassified)
3. Return nil (nothing to assign)

Never pick from blocked/archived.

## Sort Algorithm

### Primary Sort: Priority

```go
func sortByPriority(changes []Change) {
  sort.Slice(changes, func(i, j int) bool {
    pri_i := changes[i].Metadata.Priority
    if pri_i == nil {
      pri_i = ptr(0)
    }
    pri_j := changes[j].Metadata.Priority
    if pri_j == nil {
      pri_j = ptr(0)
    }
    
    // Higher priority first
    if *pri_i != *pri_j {
      return *pri_i > *pri_j
    }
    
    // Tie: older created date first (FIFO for same priority)
    return changes[i].Created.Before(changes[j].Created)
  })
}
```

### Secondary Sort: Creation Date

If two changes have the same priority, prefer the older one (FIFO within priority tier).

```
Priority  Created     Pick?
90        2026-07-10  1 ← Higher priority, pick first
90        2026-07-15  2 ← Same priority, older created
50        2026-07-01  3 ← Lower priority, skip
50        2026-07-02  4
```

## Dispatcher Decision Algorithm

### Pseudo-Code

```python
def nextChange():
  # Load all changes from openspec/changes/
  all_changes = load_open_spec_changes()
  
  # Load .specsync/metadata.json for each
  for c in all_changes:
    c.metadata = load_metadata(c.dir)
  
  # Filter by assignable stage
  assignable = [c for c in all_changes if isAssignable(c)]
  
  # No work available
  if not assignable:
    log.info("no assignable changes; all are blocked/archived/complete")
    return None
  
  # Sort by priority, then creation date
  sortByPriority(assignable)
  
  # Pick the first
  return assignable[0]
```

### Dry-Run Output

```
Dispatcher Decision:
  Next change: feature-auth-rewrite
  Priority: 95 (human focus)
  Stage: backlog (ready to pick)
  Reason: priority=95, oldest with this priority
  
Available backlog:
  - feature-auth-rewrite (pri=95, created=Jul15 22:00)
  - bugfix-critical-crash (pri=80, created=Jul15 21:50)
  - refactor-db-layer (pri=50, created=Jul14 10:00)
  
Skipped (not assignable):
  - feature-deferred (stage=blocked, reason="waiting for design review")
  - old-archived-thing (stage=archived)
  - in-code-review (stage=in-review)
```

## Edge Cases

### What if priority is negative?

Currently: treat as 0 (default). Reserved for future use.

### What if priority > 100?

Clamp to 99 (max, same as focus). Warn in logs.

### What if two changes have identical priority AND created time?

Use slug as tie-breaker (alphabetical). Rare, deterministic.

### What if a change has no metadata.json yet?

Treat as stage="backlog", priority=0. This is a new change not yet classified.

### What if metadata.json is corrupted?

Log error, skip that change (safer than crashing). Continue with others.

### What if all changes are blocked?

Return nil. Supervisor logs "no unblocked work; consider unblocking a change."

### What if priority changes while agent is working?

Next dispatch picks based on new priority. Current agent continues. Priority change doesn't interrupt mid-task (configurable?).

### What if `skein focus` is called during active work?

Immediate effect: next dispatch will pick the focused change. Current agent finishes or is interrupted (configurable).

## Logging & Audit Trail

Every dispatch decision must log:

```
timestamp | event=dispatch_decision | slug=feature-x | priority=75 | reason="oldest with priority=75" | assignable_count=5 | skipped=[blocked:1, archived:2]
```

Log file: `.skein/dispatch-audit.log` (or configured path)

## Testing Strategy

### Unit Tests

```go
func TestDispatchPriority(t *testing.T) {
  // Test 1: Higher priority picked first
  changes := []Change{
    {Slug: "low", Priority: 30, Created: t1},
    {Slug: "high", Priority: 90, Created: t2},
  }
  assert(nextChange(changes).Slug == "high")
  
  // Test 2: Same priority, older first
  changes := []Change{
    {Slug: "newer", Priority: 50, Created: t2},
    {Slug: "older", Priority: 50, Created: t1},
  }
  assert(nextChange(changes).Slug == "older")
  
  // Test 3: Blocked skipped
  changes := []Change{
    {Slug: "blocked", Stage: "blocked", Priority: 99, Created: t1},
    {Slug: "backlog", Stage: "backlog", Priority: 50, Created: t1},
  }
  assert(nextChange(changes).Slug == "backlog")
  
  // Test 4: Nil priority defaults to 0
  changes := []Change{
    {Slug: "explicit", Priority: 50, Created: t1},
    {Slug: "default", Priority: nil, Created: t1},
  }
  assert(nextChange(changes).Slug == "explicit")
}
```

### Integration Tests

```go
func TestDispatchWithRealMetadata(t *testing.T) {
  // Create temp openspec directory
  // Write proposal.md + metadata.json for each change
  // Call dispatcher
  // Verify correct change picked
}
```

### Scenario Tests

1. **Focus workflow**: Human sets focus → dispatcher picks it → agent works → unfocus
2. **Blockage workflow**: Change blocked → dispatcher skips → human unblocks → dispatcher picks
3. **Priority change**: Dispatch picks A → before A starts, human bumps B priority → next dispatch still picks A (current agent), then B
4. **All blocked**: All changes blocked → dispatcher returns nil → supervisor logs and waits

## Configuration

```yaml
dispatcher:
  # Tie-breaker for same priority
  secondary_sort: creation_date  # or "slug" or "tasks_remaining"
  
  # Include active stage in pickable?
  include_active_in_queue: false  # if true, active changes can be re-assigned
  
  # Minimum priority to consider
  min_priority: 0  # skip below this (not recommended)
  
  # Allow negative priorities?
  allow_negative_priority: false
  
  # Audit logging
  audit_log: .skein/dispatch-audit.log
  audit_level: info  # or debug for verbose
```

## Future Enhancements

1. **Dependency-aware dispatch**: If change A blocks change B, skip B until A complete
2. **Capacity-aware dispatch**: Don't assign if agent workload > threshold
3. **Skill-aware dispatch**: Only assign if agent has required skills (language, domain)
4. **Time-based decay**: Priority decreases over time (old high-priority work ages, moves down)
5. **ML-based prioritization**: LLM reads change description, recommends priority
