# Workflow State Management for Agents

This guide explains how to use specsync's workflow state system (stages, priorities, board reconciliation) effectively when working with OpenSpec changes.

## Core Concepts

### Stages (Workflow State)

Every change has a `stage` that describes where it is in the workflow:

| Stage | Meaning | When to Use |
|-------|---------|------------|
| `backlog` | Queued, ready for work | Default; ready to start |
| `active` | Currently being worked | Assign to an agent; work in progress |
| `blocked` | Blocked on external dependency | Waiting for clarification, approval, or external work |
| `in-review` | Awaiting review (code review, security review, etc.) | Submitted for feedback |
| `complete` | All tasks done, spec validated | Ready to close/archive |
| `archived` | Closed/no longer active | Done, won't reopen |

### Priority (1-100)

Optional numeric priority. Higher = more urgent.
- **90+**: Critical, unblock others
- **70-89**: High, schedule soon
- **50-69**: Medium, normal priority
- **30-49**: Low, defer if needed
- **1-29**: Very low, work when clearing backlog

## CLI Commands

### List Changes with State

```bash
# List all changes as table
specsync changes

# Filter by stage
specsync changes --stage active
specsync changes --stage blocked

# JSON output (for scripting/agents)
specsync changes --output json
specsync changes --stage active --output json
```

### Set Stage

```bash
# Move a change to a stage
specsync set-stage my-change active
specsync set-stage my-change in-review

# Use "auto" to unset (remove manual override, use derived state)
specsync set-stage my-change auto
```

### Set Priority

```bash
# Assign priority (1-100)
specsync set-priority my-change 75

# Unset priority
specsync set-priority my-change unset
```

## For AI Agents (Skein)

### Reading Current State

Before deciding what to work on next, query the change state:

```bash
specsync changes --stage backlog --output json | jq '.[] | {slug, priority, stage}'
```

**Interpret the output:**
- `priority == null` → Use natural order (older first)
- `priority > 0` → Use priority to sort work
- `stage == "blocked"` → Don't assign; wait for unblock
- `stage == "active"` → Already assigned; check progress

### Setting Stage During Work

As an agent works on a change:

```bash
# When starting
specsync set-stage my-change active

# When waiting for external input
specsync set-stage my-change blocked

# When done with implementation, ready for review
specsync set-stage my-change in-review

# When all tasks checked and validated
specsync set-stage my-change complete
```

### Using Priority for Dispatch

Skein's dispatcher should consult priority when multiple changes are available:

```bash
# Pseudo-code for agent logic
changes = fetch active changes with priority
changes = sort by (priority DESC, created ASC)
next_change = changes[0]
assign(next_change)
```

## Board Reconciliation (Three-Way Merge)

When specsync syncs to a GitHub Projects board:

### Human-Move Detection

If you manually move a card on the board (e.g., from "In Progress" to "Done"):
- specsync **will not** clobber it on next sync
- The card stays where you moved it
- A future `specsync changes` will show the local stage, not the board position

**When to use manual board moves:**
- Quick status updates without CLI
- Emergency escalation (move to "Blocked" if something is urgent)
- Real-time visibility in GitHub UI

### Conflict Avoidance

To prevent conflicts:
1. **Use CLI when programmatic**: `specsync set-stage` is authoritative
2. **Use board UI when ad-hoc**: Manual moves are respected
3. **Don't do both simultaneously**: One wins; the other is skipped with a reason

### Understanding Board Plan Output

After `specsync sync`, check the plan for status updates:

```json
{
  "StatusSkipped": "human moved the card on the board; specsync won't clobber it"
}
```

This means: **human edit detected and preserved** ✅

## Metadata Storage

State is stored in `.specsync/metadata.json` (committed to repo):

```json
{
  "version": 1,
  "stage": "active",
  "priority": 75
}
```

Board bindings are stored in `.specsync/board.json` (gitignored, disposable):

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

## Best Practices

### For Agents (Skein)

1. **Query before assigning**: Always fetch current state to check stage + priority
2. **Set stage early**: Mark as `active` when picking up work
3. **Report blockers**: Use `blocked` stage + add context in tasks
4. **Respect priority**: Sort backlog by priority; don't override agent preferences
5. **Watch for conflicts**: If board shows human-moved status, consult human before next push

### For Humans

1. **Use CLI for bulk/scripted changes**: `specsync set-stage` when updating many
2. **Use board UI for quick ad-hoc changes**: Manual moves are safe and respected
3. **Set priority upfront**: Help agents dispatch work effectively
4. **Archive aggressively**: Move done work out of view with `archived` stage
5. **Monitor blockers**: Unblock `blocked` changes as external deps resolve

### For Integration

1. **Before custom dispatching**: Verify stage is not `archived` or `blocked`
2. **After work**: Update stage to reflect real status (active → in-review → complete)
3. **On conflicts**: Log the human-move reason and skip; don't retry
4. **Persist metadata**: Commit `.specsync/metadata.json` to track decisions over time

## Troubleshooting

### "I moved a card on the board but specsync keeps changing it back"

This should not happen. If it does:
1. Check if `.specsync/board.json` exists and is readable
2. Verify human-move detection ran: `specsync sync --dry-run` should show StatusSkipped
3. If not skipping, the prior base state may be stale; delete `.specsync/board.json` and re-sync

### "My priority/stage isn't being used"

1. Run `specsync set-stage` or `specsync set-priority` to set it explicitly
2. Verify `.specsync/metadata.json` exists and is committed
3. Check `specsync changes --output json` to see current value

### "I want to override a stage Skein set"

Use `specsync set-stage my-change <new-stage>` to override any automatic derivation.

## Further Reading

- `openspec/` — OpenSpec proposal and spec files
- `.specsync/metadata.json` — This change's workflow state (committed)
- `.specsync/board.json` — Board binding cache (gitignored, regenerated)
- `cmd/specsync/main.go` — CLI implementation
- `board.go` — Three-way merge and board reconciliation logic
