# Implementation Tasks

## Phase 1: Foundation — Queue & Dispatcher (Week 1-2)

### Dispatcher Priority Logic
- [ ] Read .specsync/metadata.json in loadOpenSpecChanges()
- [ ] Parse priority field (null → 0, integer 1-100)
- [ ] Implement sortByPriority() function
- [ ] Add stage filtering: skip blocked/archived unless explicitly queried
- [ ] Test: dispatch picks higher-priority change first
- [ ] Test: equal priority → older created date wins

### Queue Display Enhancement
- [ ] Extend skein queue output columns: add PRI, P_SOURCE, STAGE
- [ ] Parse .specsync/metadata.json for each change
- [ ] Determine P_SOURCE: manual / derived / default
- [ ] Format output with proper alignment
- [ ] Add --stage flag to filter queue by stage
- [ ] Test: `skein queue --stage active` shows only active
- [ ] Test: `skein queue --sort priority` sorts descending

### Dry-Run Dispatcher (No State Change)
- [ ] Create newDispatchDecision struct with reason
- [ ] Log dispatcher picks: "picked feature-x (priority=75, 5 active jobs)"
- [ ] Add --dry-run flag to supervisor for testing
- [ ] Test: confirm priority-based picking without mutation

---

## Phase 2: Agent Integration (Week 2-3)

### Agent Hooks Infrastructure
- [ ] Define AgentHook interface in skein/supervisor
- [ ] Implement hook types: onAssign, onProgress, onSubmitReview, onComplete, onBlock
- [ ] Load hooks from .skein/config.yaml
- [ ] Execute hooks as strings (bash + env vars)
- [ ] Test: hook runs when agent assigned
- [ ] Test: environment variables available (SLUG, CHANGE_DIR, etc.)

### Hook Execution & Error Handling
- [ ] Capture hook output and errors
- [ ] Log hook execution to audit trail
- [ ] If hook fails: log warning, continue (don't block agent)
- [ ] Timeout hooks after 30s (configurable)
- [ ] Test: failed hook doesn't crash supervisor

### Update Existing Agents
- [ ] Add on_assign hook to coder agent template
- [ ] Add on_block hook to coder/debugger agents
- [ ] Add on_submit_review hook to coder agent
- [ ] Add on_complete hook to reviewer agent
- [ ] Test: hooks fire during real agent runs

### Create specsync-workflow Skill
- [ ] Implement `status` command: read and display current priority/stage
- [ ] Implement `focus-next` command: find and report next high-priority backlog change
- [ ] Implement `mark-blocked` command: set stage to blocked + add reason
- [ ] Implement `mark-review` command: set stage to in-review
- [ ] Implement `mark-complete` command: set stage to complete
- [ ] Test: skill outputs are parseable JSON

---

## Phase 3: Human Commands (Week 3)

### skein focus Command
- [ ] Implement `skein focus <slug>` command
- [ ] Set priority to 99 (maximum)
- [ ] Log decision to audit trail with timestamp
- [ ] Ensure stage is backlog (ready to pick)
- [ ] Test: focused change picked first on next dispatch
- [ ] Test: audit log records focus event

### skein unfocus Command
- [ ] Implement `skein unfocus <slug>` command
- [ ] Reset priority to 0 (or derived if available)
- [ ] Log decision to audit trail
- [ ] Test: unfocused change returns to normal sort order

### skein block Command
- [ ] Implement `skein block <slug> <reason>` command
- [ ] Set stage to blocked
- [ ] Store reason in tasks.md as checklist item
- [ ] Log decision to audit trail
- [ ] Test: blocked change skipped in dispatch
- [ ] Test: reason visible in tasks.md

### skein unblock Command
- [ ] Implement `skein unblock <slug>` command
- [ ] Set stage back to backlog
- [ ] Log decision to audit trail
- [ ] Test: unblocked change available for dispatch

### Integration Tests
- [ ] Test: focus → dispatch picks it → unfocus → normal sort
- [ ] Test: block → dispatch skips it → unblock → available again
- [ ] Test: focus + block → block wins (skipped)

---

## Phase 4: Board Automation (Week 4-5)

### Background Board Sync Task
- [ ] Implement backgroundBoardSync() in supervisor main loop
- [ ] Read all changes every 5 minutes (configurable)
- [ ] For each change: check if has ref + board configured
- [ ] Call specsync sync (or sync --dry-run for preview)
- [ ] Log results: pushed / skipped (reason) / error
- [ ] Test: board updates on schedule
- [ ] Test: human-move detection works (from Phase 3 board.go)

### File Watcher Trigger
- [ ] Watch .specsync/metadata.json for changes
- [ ] On change: immediately trigger sync (not on schedule)
- [ ] Test: priority change → board updates within 1s
- [ ] Test: stage change → board updates within 1s

### Conflict Reporting
- [ ] Parse sync plan: detect StatusSkipped reason
- [ ] If StatusSkipped contains "human moved": log as human-move
- [ ] If StatusSkipped contains "conflict": log as conflict, escalate?
- [ ] Test: human-move logged correctly
- [ ] Test: conflict detected and reported

### Board State Audit
- [ ] Add command: `skein specsync audit` to show sync history
- [ ] Parse .specsync/board.json bindings
- [ ] Display last sync time, local base, remote base
- [ ] Show human-move history
- [ ] Test: audit output readable and useful

---

## Phase 5: Configuration & Documentation (Week 5-6)

### Config Schema
- [ ] Define new specsync section in .skein/config.yaml
- [ ] Add: enabled, board_sync_interval, auto_sync_on_metadata_change
- [ ] Add: audit_log path, priority_model, blocked_behavior
- [ ] Add: board conflict_strategy
- [ ] Validate config on startup
- [ ] Test: invalid config rejected

### Agent Hooks Configuration
- [ ] Document hook format in .skein/config.yaml reference
- [ ] Add example hooks for all types
- [ ] Test: hooks loaded and parsed correctly

### Migration Tooling
- [ ] Implement `skein migrate specsync --auto-prioritize`
- [ ] Implement `skein migrate specsync --clear`
- [ ] Test: backfill works without data loss
- [ ] Test: clear resets safely

### Documentation
- [ ] Update WORKFLOW.md with Skein dispatch explanation
- [ ] Create `.claude/CLAUDE.md` section: how agents use priority
- [ ] Document skein focus/block commands
- [ ] Document audit logging
- [ ] Create troubleshooting guide: "priority not affecting dispatch"

---

## Phase 6: Integration Tests & Polish (Week 6)

### End-to-End Tests
- [ ] Test: `specsync set-priority` → next dispatch affected
- [ ] Test: `skein focus` → agent assigned → hook fires → stage changes
- [ ] Test: board conflict → human-move detected → logged
- [ ] Test: full workflow: backlog → active → in-review → complete
- [ ] Test: agent blocked → dispatch skips → human unblocks → dispatch resumes

### Performance
- [ ] Measure: queue load time with 100+ changes
- [ ] Measure: dispatcher decision time (< 100ms)
- [ ] Measure: background sync overhead (<5% CPU during sync)
- [ ] Test: no memory leaks on long runs

### Observability
- [ ] Add priority metrics: average, distribution, changes/min
- [ ] Add dispatch metrics: picks/min, avg priority of picked
- [ ] Add board metrics: syncs/min, human-moves/min, conflicts/min
- [ ] Create dashboard: show active priorities, blocked changes, sync health

### Documentation Polish
- [ ] Review all docs for completeness
- [ ] Add architecture diagrams
- [ ] Add examples: common scenarios
- [ ] Create troubleshooting: "Why isn't my priority working?"

---

## Meta Tasks

### Coordination
- [ ] Link related OpenSpec changes (e.g., board-state-reconciliation)
- [ ] Create roadmap showing phases
- [ ] Identify blocker dependencies (e.g., Phase 2 needs specsync-workflow skill)

### Review Gates
- [ ] Code review: dispatcher logic correct?
- [ ] Security review: no injection vectors in hook execution?
- [ ] Performance review: dispatch time acceptable?
- [ ] UX review: CLI commands intuitive?

### Release Checklist
- [ ] All tests passing
- [ ] Performance benchmarks met
- [ ] Documentation complete
- [ ] Backward compatibility verified (old configs still work)
- [ ] Migration path tested
- [ ] Release notes written
