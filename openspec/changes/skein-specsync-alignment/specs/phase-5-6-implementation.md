# Phase 5-6 Implementation Guide

## Phase 5: Configuration & Documentation (Week 5-6)

This phase solidifies the integration with configuration, migration tooling, and documentation.

### 5.1 Configuration Schema Validation

**File**: `internal/config/specsync_schema.go`

**Status**: ✅ DONE (basic schema written, ready for integration)

**What's there**:
- `SpecSyncConfig` struct with three main sections:
  - `Board` — background sync interval, auto-sync on metadata change, conflict strategy
  - `Dispatcher` — enabled, include active, secondary sort, min priority
  - Core settings — enabled, priority model, blocked behavior, audit log

- Validation methods:
  - `Validate()` checks all config values are sensible
  - Type-safe defaults via `DefaultSpecSyncConfig()`
  - Merge logic for overlaying user config on defaults

**Still needed**:
- [ ] Integration into `.skein/config.yaml` parsing (Phase 1-2 code loads this)
- [ ] Validation on supervisor startup (fail-fast for bad config)
- [ ] CLI command: `skein config validate` to check config without starting

### 5.2 Migration Tooling

**File**: `internal/cli/migrate_specsync.go`

**Status**: ✅ DONE (two strategies implemented and tested)

**What's there**:
- `MigrateSpecSync()` with two strategies:
  1. **auto-prioritize**: Estimate priority from task count (more tasks = higher priority) and creation date (older = higher priority)
  2. **clear**: Reset all to priority=0, stage=backlog for manual reprioritization

- Implementation:
  - Atomic writes (temp + rename)
  - Dry-run mode for preview
  - Detailed output showing what would change

**Commands to expose**:
```bash
skein migrate specsync --auto-prioritize [--dry-run]
skein migrate specsync --clear [--dry-run]
```

**Still needed**:
- [ ] Wire into CLI command handler
- [ ] Integration tests (run migration, verify metadata.json written)
- [ ] Documentation on when to use each strategy

### 5.3 Agent Hook Configuration

**File**: `.skein/config.yaml` (new specsync section)

**What needs to be defined**:
```yaml
specsync:
  enabled: true
  
  # Board automation
  board:
    enabled: true
    sync_interval: 5m
    auto_sync_on_metadata_change: true
    conflict_strategy: report  # or "favor_local", "prompt_human"
    max_retries: 3
  
  # Dispatcher settings
  dispatcher:
    enabled: true
    include_active_in_queue: false
    secondary_sort: creation_date  # or "slug"
    min_priority: 0
    allow_negative_priority: false
  
  # Priority interpretation
  priority_model: specsync  # or "p1-p5", "hybrid"
  
  # Blocked change behavior
  blocked_behavior: skip  # or "wait_with_timeout", "escalate"
  
  # Audit logging
  audit_log: .skein/specsync-audit.log
  
  # Metadata loading
  metadata_strategy: lenient  # or "strict"

# Agent hooks (already partially done by Agent 1-2)
agent_hooks:
  on_assign:
    - specsync set-stage $(SLUG) active
  
  on_block:
    - specsync set-stage $(SLUG) blocked
    # Agent adds reason to tasks.md
  
  on_submit_review:
    - specsync set-stage $(SLUG) in-review
    - /log-to-slack "PR ready: $(PR_URL)"
  
  on_complete:
    - specsync set-stage $(SLUG) complete
    - /telegram-notify "✅ $(SLUG) complete"
```

**Still needed**:
- [ ] Parse this config in Skein's config loader
- [ ] Validate against SpecSyncConfig schema
- [ ] Make available to supervisor and dispatcher

### 5.4 Documentation Updates

**Files**:
- `WORKFLOW.md` — Update with Skein integration
- `.claude/CLAUDE.md` — Update with focus/block semantics
- `openspec/changes/skein-specsync-alignment/IMPLEMENTATION.md` — How to run Phase 1-6

**What to update in WORKFLOW.md**:
- Add section: "Skein Integration & Auto-Dispatch"
- Explain how priority affects Skein's dispatcher
- Document `skein focus`, `skein block`, `skein unblock` workflow
- Update agent lifecycle section with hook execution

**What to add to .claude/CLAUDE.md**:
- "Automatic state management": agents call hooks automatically
- "Blocking work": how to use `skein block` when hitting external dependencies
- "Focus workflow": complete example of focusing a change
- "Board automation": how specsync syncs to GitHub automatically

**What to create in IMPLEMENTATION.md**:
- 6-phase timeline
- Per-phase checklist
- How to test each phase
- Rollback procedure (if something goes wrong)

### 5.5 Testing & Quality Assurance

**Unit Tests** (for config schema):
- [ ] `TestSpecSyncConfigValidate_*` — validation logic
- [ ] `TestDefaultSpecSyncConfig` — sensible defaults
- [ ] `TestMergeConfig_*` — overlay logic

**Integration Tests**:
- [ ] `TestMigrateAutoPrioritize_*` — migration works end-to-end
- [ ] `TestConfigLoads_*` — config parses from YAML correctly
- [ ] `TestHooksExecute_*` — hooks fire in correct order (Agent 1-2 already covers this)
- [ ] `TestBoardSyncAutomation_*` — background sync task works
- [ ] `TestFileWatcherTrigger_*` — metadata change triggers immediate sync

**Observability**:
- [ ] Audit logging of all priority/stage/sync decisions
- [ ] Metrics: priority distribution, dispatch frequency, board sync health
- [ ] Dashboard showing real-time status

---

## Phase 6: Integration & Production Polish (Week 6)

### 6.1 End-to-End Workflow Tests

**Scenario 1: Focus & Dispatch**
```
1. Create change "feature-x"
2. Run: skein focus feature-x
3. Verify: priority set to 99 in .specsync/metadata.json
4. Verify: Next dispatch picks feature-x
5. Verify: Agent assigned, on_assign hook fires
6. Verify: Stage changed to "active"
```

**Scenario 2: Block & Resume**
```
1. Agent working on change, hits blocker
2. Agent calls: skein block feature-x "waiting for API team"
3. Verify: Stage set to "blocked" in metadata
4. Verify: Reason added to tasks.md
5. Verify: Dispatcher skips this change on next cycle
6. Human calls: skein unblock feature-x
7. Verify: Stage set to "backlog"
8. Verify: Dispatcher picks it again when ready
```

**Scenario 3: Full Workflow**
```
1. Human: skein focus feature-security
2. Dispatcher picks it (priority=99)
3. Coder assigned, on_assign hook fires
4. Coder works, makes commits
5. Coder submits PR, on_submit_review hook fires
6. Stage: active → in-review
7. Board syncs (three-way merge respects any human moves)
8. Reviewer reviews, approves
9. Coder merges, on_complete hook fires
10. Stage: in-review → complete
11. Verify: Board updated, audit log shows full progression
```

**Scenario 4: Board Conflict Handling**
```
1. Change has stage=active on local, status=In Progress on board
2. Human manually moves card to Done on board
3. Supervisor's background sync detects: remote changed, local didn't
4. Decision: report-remote-move, don't clobber
5. Verify: Card stays at Done on board
6. Verify: Audit log shows "human moved card; skipped update"
7. Verify: Local stage still says active (not synced down)
```

### 6.2 Performance Testing

**Dispatcher Decision Time**
- Baseline: Should pick next change in < 100ms
- Load: With 100+ changes, still < 100ms
- Metric: Measure and log dispatcher latency

**Board Sync Overhead**
- Baseline: Background sync < 5% CPU during operation
- Load: Multiple syncs queued up
- Metric: Measure sync duration and frequency

**Memory Usage**
- No leaks on 24-hour run
- Metric: Monitor memory over time

### 6.3 Observability & Monitoring

**Audit Log Format**
```
timestamp | event=dispatch_decision | slug=feature-x | priority=99 | reason="human focus" | assignable_count=5
timestamp | event=hook_executed | hook=on_assign | slug=feature-x | status=success | duration_ms=150
timestamp | event=board_sync | slug=feature-x | action=pushed | status=In\ Progress
timestamp | event=board_sync | slug=feature-y | action=skipped | reason="human moved the card"
```

**Metrics to Track**
- Dispatch decisions per hour
- Average priority of dispatched changes
- Hook success rate (%)
- Board sync frequency, success rate, conflicts detected
- Human moves detected per day
- Time from focus to assignment (average)

**Dashboard** (optional, can be added later)
- Active changes by stage
- Priority distribution
- Board sync health
- Hook execution timeline
- Recent audit log entries

### 6.4 Documentation & Release

**README Updates**
- Link to WORKFLOW.md
- Quick start: "How to prioritize work"
- "How to block/unblock"
- Troubleshooting section

**Release Notes for v0.8.0** (hypothetical)
```
## Skein-SpecSync Alignment (Priority-Driven Dispatch)

### Features
- Dispatcher now reads .specsync/metadata.json priority field
- `skein focus <change>` immediately prioritizes work
- `skein block <change> <reason>` marks blocked, skips in dispatch
- `skein unblock <change>` resumes work
- Agents automatically update specsync state via hooks
- Board sync fully automated with human-move detection

### Migration
- Run: `skein migrate specsync --auto-prioritize` to backfill priorities
- Or: `skein migrate specsync --clear` for manual reprioritization

### Backward Compatibility
- Old changes without .specsync/metadata.json default to priority=0, stage=backlog
- All existing Skein configs still work (specsync integration is opt-in)
```

---

## Implementation Checklist for Phase 5-6

### Configuration (Phase 5)
- [ ] Config schema written and validated
- [ ] Integration into Skein's config loader
- [ ] Validation on startup (fail-fast)
- [ ] `skein config validate` command
- [ ] Documentation of all options

### Migration (Phase 5)
- [ ] Both strategies implemented (auto-prioritize, clear)
- [ ] Dry-run mode works
- [ ] CLI command: `skein migrate specsync`
- [ ] Integration tests
- [ ] Documentation on when to use each

### Agent Hooks Configuration (Phase 5)
- [ ] Example hooks in `.skein/config.yaml`
- [ ] Per-role hook overrides work
- [ ] Hooks can be disabled individually
- [ ] Documentation

### Documentation (Phase 5)
- [ ] WORKFLOW.md updated with Skein section
- [ ] .claude/CLAUDE.md updated with new commands
- [ ] IMPLEMENTATION.md created with timeline
- [ ] Troubleshooting guide
- [ ] API/integration docs for custom hooks

### Testing (Phase 6)
- [ ] All unit tests pass
- [ ] Integration tests for full workflows
- [ ] E2E tests covering all scenarios
- [ ] Performance benchmarks met
- [ ] Memory leak tests pass

### Observability (Phase 6)
- [ ] Audit logging implemented
- [ ] Metrics collection working
- [ ] Dashboard optional (can be added in Phase 7)

### Release (Phase 6)
- [ ] Release notes written
- [ ] Backward compatibility verified
- [ ] Migration guides clear
- [ ] Rollback procedure documented

---

## Known Unknowns / Future Decisions

### Configuration Conflicts
- If user sets priority to 50 AND stage to blocked, which wins? (Blocked wins; skipped regardless of priority)
- If priority_model is "p1-p5" but metadata has 1-100, how to reconcile? (Convert? Error? Default?)

### Board Conflict Resolution
- Current: "report" (log and skip). Future: "prompt_human" (ask via Telegram), "favor_local" (push local, clobber board)
- Should conflicts escalate to on-call? How?

### Audit Retention
- How long to keep audit logs? Rotate? Archive?
- Should audit logs feed into a dashboard or metric system?

### Performance Tuning
- Is 5-minute background sync interval right? User-configurable?
- Should file watcher sync happen immediately or batch changes?
- Should dispatcher cache metadata in memory or re-read each time?

---

## Success Criteria for Phase 5-6

✅ Configuration schema complete and validated  
✅ Migration tooling working (both strategies)  
✅ All commands integrated into CLI  
✅ Documentation comprehensive and clear  
✅ End-to-end workflows tested and working  
✅ Performance acceptable (dispatcher < 100ms, board sync < 5% CPU)  
✅ Audit logging comprehensive  
✅ Backward compatibility verified  
✅ Release notes written  

---

## Timeline Estimate

**Week 5**:
- Config schema integration (1-2 days)
- Migration tooling CLI integration (1-2 days)
- Documentation updates (2-3 days)

**Week 6**:
- End-to-end testing (2-3 days)
- Performance testing (1-2 days)
- Observability setup (1 day)
- Release prep and polish (1-2 days)

**Total**: 2 weeks for solid, production-ready Phase 5-6

---

## Next Steps

1. ✅ Spec written (this document)
2. ⏭️ Integrate config schema into Skein's config loader
3. ⏭️ Wire migration commands into CLI
4. ⏭️ Update WORKFLOW.md and .claude/CLAUDE.md
5. ⏭️ Run end-to-end tests with Agent 1-2 output
6. ⏭️ Performance testing and tuning
7. ⏭️ Release and announce v0.8.0
