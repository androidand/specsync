# Skein-SpecSync Alignment: Priority-Driven Dispatch

**Goal**: Make Skein respect OpenSpec prioritization so humans can direct focus ("work on feature X") and agents automatically manage workflow state.

## Problem

Skein currently ignores specsync's priority and stage system. It treats all changes equally, picking work arbitrarily rather than by human-set priority. This creates two problems:

1. **No focus**: Can't tell Skein "focus on the critical security fix" — it might work on unrelated refactors
2. **No state tracking**: Agents can't update workflow state as they work; stages stay empty, priorities unused
3. **Parallel systems**: specsync metadata and Skein's queue are disconnected; humans have to manage both

Result: specsync's rich workflow model sits unused while Skein makes uninformed work decisions.

## Solution Overview

Integrate specsync's priority and stage system deeply into Skein:

1. **Dispatcher reads specsync state** → sorts backlog by priority before assigning work
2. **Agents write specsync state** → automatically update stage as work progresses (active → in-review → complete)
3. **CLI priority commands affect dispatch** → `specsync set-priority` immediately changes what Skein picks next
4. **Human-focused queuing** → `skein focus <change>` sets it highest priority, all others lower
5. **Board reconciliation automatic** → Skein syncs to GitHub Projects without manual intervention

This makes specsync the **single source of truth** for work prioritization and state, with Skein as the **agent orchestrator**.

## Detailed Specification

### 1. Priority Dispatch (Dispatcher Integration)

**Current**: Skein's supervisor picks next change arbitrarily from queue.

**Desired**: Before assigning to an agent, sort by specsync priority.

#### Changes to `skein queue` and Dispatcher Logic

```go
// Pseudo-code for new dispatch decision
func nextChangeToAssign() Change {
  // Load all changes with specsync metadata
  changes := loadOpenSpecChanges()
  
  // Filter by stage: only backlog, or include active if no backlog
  available := filterByStage(changes, []Stage{backlog, active})
  
  // Sort by priority (descending), then by created (ascending)
  sort.Slice(available, func(i, j int) bool {
    pri_i := available[i].Priority ?? 0
    pri_j := available[j].Priority ?? 0
    if pri_i != pri_j {
      return pri_i > pri_j  // Higher priority first
    }
    return available[i].Created.Before(available[j].Created)  // Older first if tied
  })
  
  // Skip blocked/archived/in-review
  for _, c := range available {
    if canAssign(c) {
      return c
    }
  }
  return nil
}
```

#### New Metadata in Queue Display

`skein queue` output should show:

```
SLUG                      STATUS   PRI  TASKS  STAGE      P_SOURCE MODIFIED
feature-auth-rewrite      pending  95   5/8    backlog    manual   Jul 15 22:00
bugfix-critical-crash     pending  80   2/2    backlog    manual   Jul 15 21:50
refactor-db-layer         pending  50   12/18  backlog    derived  Jul 14 10:00
nice-to-have-feature      pending  20   0/5    backlog    default  Jul 10 05:00
```

Where `P_SOURCE` is: manual (set via CLI), derived (from tasks.md), default (fallback = 0).

### 2. State Management by Agents (Agent Integration)

**Current**: Agents work but never call `specsync set-stage`.

**Desired**: Agents automatically update specsync state as they work.

#### Agent Lifecycle Hooks

```yaml
# In .skein/config.yaml or agent templates

agent_hooks:
  on_assign:
    # When agent starts working on a change
    - specsync set-stage $(SLUG) active
    
  on_progress:
    # When agent makes substantive progress (commit, passing test)
    - specsync set-stage $(SLUG) active  # idempotent, ensures active
    
  on_submit_review:
    # When agent opens PR or submits for review
    - specsync set-stage $(SLUG) in-review
    
  on_complete:
    # When all tasks checked and validated
    - specsync set-stage $(SLUG) complete
    
  on_block:
    # When agent hits external dependency
    - specsync set-stage $(SLUG) blocked
    # Agent adds reason to tasks.md:
    # "- [ ] waiting for @human to review design doc"
```

#### Agent Skills

Create new skills agents can call:

```bash
skein skill add specsync-workflow
# Agents can now call in their routines:
# /specsync-workflow status        → get current priority/stage
# /specsync-workflow focus-next    → find and report next high-priority change
# /specsync-workflow mark-blocked  → set blocked + reason
```

### 3. Human Prioritization Commands

**Desired**: Simple CLI to express focus.

#### New/Extended Commands

```bash
# Set raw priority (1-100)
specsync set-priority feature-x 90

# Human-friendly "focus" command
skein focus feature-x
# → Sets priority to 99, all others relative to this one stay same but lower
# → Forces stage to backlog (ready to pick up)
# → Logs decision to audit trail

# Unfocus (clear priority)
skein unfocus feature-x
# → Resets to default (0) or derived priority

# Block a change (waiting on something)
skein block feature-x "waiting for design review"
# → Calls specsync set-stage feature-x blocked
# → Adds reason to tasks.md
# → Skein won't pick it until unblocked

# Unblock
skein unblock feature-x
# → Moves back to backlog
```

### 4. Board Sync Automation

**Current**: Manual `specsync sync` or GitHub workflow.

**Desired**: Automatic syncing whenever specsync state changes, with human-move detection.

#### Supervisor Background Task

```go
// In Skein supervisor's main loop
func backgroundBoardSync() {
  ticker := time.NewTicker(5 * time.Minute)  // Configurable
  for range ticker.C {
    changes := loadOpenSpecChanges()
    for _, c := range changes {
      if c.HasRef() && target.Configured() {
        plan, err := specsyncSync(c)
        if err != nil {
          logWarning("board sync failed for %s: %v", c.Slug, err)
          continue
        }
        
        // Log what happened
        if plan.StatusSkipped != "" {
          logInfo("board: %s - %s", c.Slug, plan.StatusSkipped)
        }
        if plan.StatusName != "" {
          logInfo("board: %s - updated to %s", c.Slug, plan.StatusName)
        }
      }
    }
  }
}
```

Or trigger on specsync state change:

```bash
# Hook: specsync set-stage fires → Skein detects file change → auto-syncs
specsync set-stage my-change active
# → .specsync/metadata.json updated
# → Skein's file watcher detects change
# → Triggers `specsync sync --dry-run` (preview)
# → Logs decision (pushed / skipped human-move / conflict)
```

### 5. Priority Interpretation

#### P1-P5 → specsync 1-100 Mapping (Optional)

For compatibility with Skein's native P1-P5:

```
skein prioritize my-change P1  →  specsync set-priority my-change 90
skein prioritize my-change P2  →  specsync set-priority my-change 70
skein prioritize my-change P3  →  specsync set-priority my-change 50
skein prioritize my-change P4  →  specsync set-priority my-change 30
skein prioritize my-change P5  →  specsync set-priority my-change 10
```

Or allow both systems to coexist:
- **specsync priority**: Fine-grained (1-100), committed, stable
- **Skein P1-P5**: Temporary session state, useful for real-time tuning

### 6. Data Flow Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    OpenSpec Changes                          │
│                  openspec/changes/<slug>/                   │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ├─ proposal.md (spec)
                        ├─ tasks.md (work items)
                        ├─ links.md (cross-refs)
                        └─ .specsync/
                           ├─ metadata.json ← PRIORITY & STAGE (source of truth)
                           ├─ refs.json (cached GitHub issue ref)
                           └─ board.json (cached board binding for 3-way merge)
                        
                ╭───────────┴──────────╮
                │                       │
         ┌──────▼────────┐      ┌──────▼────────┐
         │  Skein Queue  │      │ GitHub Projects
         │  (sessions)   │      │  Board
         └──────┬────────┘      └──────┬────────┘
                │                       │
    specsync reads for:      specsync writes to:
    - Priority dispatch      - Issue sync (existing)
    - Stage filtering        - Project card status (new)
                             - Human-move detection (new)
                
                ╭────────────┬────────────╮
                │            │            │
         ┌──────▼──────┐ ┌───▼────┐ ┌───▼──────┐
         │  Coder      │ │Debugger│ │Reviewer  │
         │  Agent      │ │ Agent  │ │ Agent    │
         └─────┬───────┘ └───┬────┘ └───┬──────┘
               │             │          │
     Call on_assign hook  on_block  on_submit_review
         ↓             ↓           ↓
    specsync set-stage active/blocked/in-review
         │             │          │
         └──────────────┼──────────┘
                        │
              .specsync/metadata.json updated
                        │
                Skein's file watcher detects
                        │
         Trigger background board sync
                        │
           .specsync/board.json updated
           (three-way merge handles human moves)
```

### 7. Implementation Phases

#### Phase 1: Foundation (1-2 weeks)
- [ ] Extend `skein queue` to read .specsync/metadata.json
- [ ] Display priority and stage in queue output
- [ ] Implement priority-based sorting in dispatcher (dry-run only)
- [ ] Test with manual specsync state changes

#### Phase 2: Agent Integration (2-3 weeks)
- [ ] Add agent hooks: `on_assign`, `on_block`, `on_submit_review`, `on_complete`
- [ ] Create `specsync-workflow` skill with state queries
- [ ] Update coder/debugger/reviewer agents to call hooks
- [ ] Test workflow progression with real agent runs

#### Phase 3: Human Commands (1 week)
- [ ] Implement `skein focus <slug>`
- [ ] Implement `skein block <slug> <reason>`
- [ ] Implement `skein unblock <slug>`
- [ ] Test prioritization immediate effect on next dispatch

#### Phase 4: Board Automation (1-2 weeks)
- [ ] Add background sync task to supervisor
- [ ] Implement file watcher trigger for metadata changes
- [ ] Test three-way merge conflict detection
- [ ] Document board conflict resolution workflow

#### Phase 5: Polish & Observability (1 week)
- [ ] Add priority audit logging
- [ ] Extend `skein board` to show priority/stage
- [ ] Create observability dashboard
- [ ] Comprehensive integration tests

### 8. Configuration

New `.skein/config.yaml` section:

```yaml
specsync:
  # Enable specsync priority dispatch
  enabled: true
  
  # How often to sync board state in background
  board_sync_interval: 5m
  
  # File watcher triggers immediate sync on metadata change
  auto_sync_on_metadata_change: true
  
  # Audit logging
  audit_log: .skein/specsync-audit.log
  
  # Priority interpretation
  priority_model: specsync  # or "p1-p5" for Skein compat, or "hybrid"
  
  # Blocked change behavior
  blocked_behavior: skip    # skip immediately, or "wait-with-timeout"
  
  # Board reconciliation
  board:
    enabled: true
    conflict_strategy: report  # "report", "prompt-human", or "favor-local"
    background_sync: true
```

### 9. Migration & Adoption

#### Existing Changes

For changes already in queue:

```bash
# Option 1: Backfill priorities based on task count
skein migrate specsync --auto-prioritize
# Changes with more tasks → higher priority
# Changes with less → lower priority

# Option 2: Assign all to backlog, let humans prioritize
skein migrate specsync --clear
# Resets all to priority=0, stage=backlog
```

#### New Workflows

1. Human creates change with `specsync create` (or via proposal.md)
2. Human runs `specsync set-priority <slug> 75` to indicate importance
3. Skein reads priority on next queue update
4. When agent assigned, calls `specsync set-stage <slug> active`
5. As agent works, stage updates (active → in-review → complete)
6. specsync syncs to board with human-move detection

### 10. Success Criteria

- ✅ `skein focus feature-x` immediately affects next dispatch decision
- ✅ Queue output shows priority and stage, sorted by priority
- ✅ Agents update specsync state without manual intervention
- ✅ Human board moves are preserved (not clobbered)
- ✅ `specsync set-priority` changes take effect on next supervisor cycle
- ✅ All changes with `stage=blocked` are skipped until unblocked
- ✅ Audit log shows all priority/stage changes with reasons
- ✅ Integration tests cover dispatch, state progression, board sync

## Files to Modify

- `skein/supervisor/dispatch.go` — Dispatcher priority logic
- `skein/queue/queue.go` — Queue display and sorting
- `skein/commands/focus.go` — New focus/unfocus/block commands
- `skein/commands/queue.go` — Extend to show priority/stage
- `skein/supervisor/background.go` — Background board sync
- `skein/config/schema.go` — New specsync config section
- `.skein/config.yaml` — New config values
- `.skein/agents/*.yaml` — Agent hooks configuration
- Tests: `*_test.go` for all above

## Skills & Automations to Create

- `specsync-dispatch` — Agent skill to query and report next priority change
- `specsync-workflow` — Agent skill to update state (block, mark-review, etc.)
- `board-sync` — Skill to manually trigger board reconciliation
- `priority-audit` — Skill to show priority history

## Open Questions

1. **Priority inheritance**: If a change has no explicit priority, should we derive from task count (complexity) or commit velocity?
2. **Dynamic re-prioritization**: Should human-set priority fade over time (e.g., "reset to default after 48 hours") or persist indefinitely?
3. **P1-P5 coexistence**: Keep Skein's native P1-P5 for session tuning, or fully migrate to specsync 1-100?
4. **Conflict escalation**: When board and local conflict, should Skein auto-escalate to human or make a decision rule-based?
5. **Archive behavior**: Should archived changes be hidden from queue entirely or shown grayed-out?

## Future Extensions

- **Swimlanes by stage**: `skein board` grouped by backlog/active/blocked/in-review
- **Dependency awareness**: `specsync link` creates blockage relationships; defer dependent changes until blocker complete
- **Capacity planning**: `skein health` shows capacity vs. active changes; suggest unblocking or deferring based on capacity
- **AI-driven prioritization**: LLM analyzer reads change descriptions, suggests priority based on business value + complexity
- **Multi-project coordination**: Sync priorities across linked changes in different repos
