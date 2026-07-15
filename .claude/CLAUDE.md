# AI Agent Guide for specsync Workflow

Quick reference for agents (Skein, Claude Code) working with OpenSpec changes.

## TL;DR

1. **Before assigning work**: `specsync changes --stage backlog --output json` → sort by priority
2. **When starting**: `specsync set-stage <slug> active`
3. **When blocked**: `specsync set-stage <slug> blocked` (add reason in tasks)
4. **When done**: `specsync set-stage <slug> complete`
5. **Never override a human board move** — three-way merge detects and skips

## Key Commands

```bash
# List work by priority
specsync changes --stage backlog --output json | jq 'sort_by(.priority // 0) | reverse'

# Set stage (backlog, active, blocked, in-review, complete, archived)
specsync set-stage my-change active

# Set priority (1-100; higher = more urgent)
specsync set-priority my-change 75

# Unset manual override (use derived state)
specsync set-stage my-change auto
```

## Stage Meanings for Dispatch

| Stage | Agent Should | Notes |
|-------|------------|-------|
| `backlog` | Pick up if no higher priority `active` | Ready to start |
| `active` | Continue or hand off | Already being worked |
| `blocked` | Skip; come back later | Waiting on external dependency |
| `in-review` | Don't assign; wait for resolution | Under review |
| `complete` | Archive or close | All work done |
| `archived` | Ignore | Closed/won't reopen |

## Priority Logic

```
IF priority == null:
  use creation date (older first)
ELSE:
  sort by priority DESC (90+ = critical, 50-69 = normal, <30 = defer)
```

## Board Reconciliation

**You don't need to do anything.** specsync handles it:

1. specsync reads local stage from `.specsync/metadata.json`
2. Queries board for current status (if configured)
3. Compares against last-synced base state (in `.specsync/board.json`)
4. **If human moved card**: skips update, preserves human edit ✅
5. **If local changed, remote didn't**: pushes update to board ✅
6. **If both changed**: logs conflict, skips, awaits human review ⚠️

**Important**: Don't fight the board. If a human manually moved a card, `specsync sync --dry-run` will show:
```
StatusSkipped: human moved the card on the board; specsync won't clobber it
```

This is correct behavior—respect it.

## Workflow Pattern (Example)

```bash
# 1. Find next task
specsync changes --stage backlog --output json | jq '.[0]'

# 2. Assign and start
specsync set-stage my-change active

# 3. Do work (implement, test, etc.)
# ... git commits, PR, etc. ...

# 4. If you hit a blocker
specsync set-stage my-change blocked
# Add reason in tasks.md: "- [ ] waiting for @human to review design"

# 5. When implementation done, ready for review
specsync set-stage my-change in-review

# 6. When all tasks checked and validated
specsync set-stage my-change complete
```

## Reading .specsync/ Files

**`.specsync/metadata.json`** (committed, authoritative):
```json
{
  "version": 1,
  "stage": "active",
  "priority": 75
}
```
→ Use this to know the current state.

**`.specsync/board.json`** (gitignored, for local cache):
```json
{
  "version": 1,
  "bindings": {
    "owner:6:github": {
      "provider": "github",
      "project_id": "...",
      "local_stage_base": "active",
      "remote_option_id_base": "...",
      "synced_at": "2026-07-15T21:55:00Z"
    }
  }
}
```
→ Do not edit this; it's auto-generated. Used for detecting human board moves.

## Anti-Patterns

❌ **Don't**: `specsync set-stage my-change active` then immediately sync without checking stage again  
✅ **Do**: Query state, decide based on priority, then assign

❌ **Don't**: Assume board and local state are always in sync  
✅ **Do**: Run `specsync sync --dry-run` to see what would happen

❌ **Don't**: Manually edit `.specsync/board.json`  
✅ **Do**: Let specsync manage it; it's regenerated on each sync

❌ **Don't**: Fight a human board move by retrying with force  
✅ **Do**: Recognize the conflict and report it (via PR comment or Telegram)

## Troubleshooting

**"I set a stage but `specsync changes` shows something else"**
→ Stage may be derived from tasks.md (if all tasks checked → complete). Check the proposal.

**"specsync sync shows StatusSkipped for a board move"**
→ This is correct. Human moved card; specsync won't override.

**"I want to prioritize this change but priority is null"**
→ Run: `specsync set-priority my-change 80`

## Files to Know

- `WORKFLOW.md` — Full workflow documentation
- `openspec/changes/` — Individual change proposals
- `.specsync/metadata.json` — This change's workflow state
- `.specsync/board.json` — Board binding cache (auto-managed)
- `cmd/specsync/main.go` — CLI source
- `board.go` — Three-way merge logic

**Start here**: `specsync changes --output json` to see all work.
