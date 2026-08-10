# Claude Code Configuration

This is the Claude Code project configuration for specsync development.

**Full agent guidelines**: See [AGENTS.md](../AGENTS.md) in the repository root.

## Quick Commands

```bash
# Find work by priority
specsync changes --stage backlog --json | jq 'sort_by(.priority // 0) | reverse'

# Start work
specsync set-stage <slug> active
specsync -change <slug>

# If blocked
specsync set-stage <slug> blocked

# When done
specsync set-stage <slug> complete
specsync -change <slug>
```

## Key Concepts

- **Stages**: backlog → active → (blocked if needed) → in-review → complete → archived
- **Three-way merge**: specsync respects human board moves (won't clobber them)
- **Single source**: OpenSpec changes drive GitHub issues, not the reverse
- **Dogfooding**: All work tracked as specs; commits must reference issues

## Before You Start

Read [AGENTS.md](../AGENTS.md) for complete guidance on:
- Workflow principles and rules
- Security and dogfooding standards
- PR and branch conventions
- Commit message standards
- How to plan, implement, and complete changes

## References

- **Workflow**: See AGENTS.md #Working Paths
- **PR conventions**: See AGENTS.md #Branches, Worktrees & PRs
- **Dogfooding rules**: See AGENTS.md #Dogfooding
- **Available commands**: `specsync agent-help` or `specsync changes --json`
- **Diagnostics**: `specsync doctor`
