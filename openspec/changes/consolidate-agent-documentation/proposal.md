# Consolidate agent documentation to reduce duplication

## Context

Agent guidance is currently split across three files with significant overlap:

- **AGENTS.md** (144 lines) — agent workflow, rules, dogfooding, security, branching
- **CLAUDE.md** (62 lines) — brief guide, problem statement
- **.claude/CLAUDE.md** (165 lines) — AI agent guide, commands, stage meanings, board reconciliation

This creates confusion:
- Which is the source of truth?
- What's the difference between them?
- Which should agents read?
- Why is information repeated?

Result: agents read multiple files, prompt bloat, maintenance burden.

## Proposed Changes

### 1. Establish Single Source of Truth

**AGENTS.md** is the authoritative guide. It covers:

- Principles and philosophy
- Dogfooding rules and verification
- Security and data handling
- Working paths and workflows
- Branching and PR conventions
- Commit message standards
- Documentation style

### 2. Consolidate Content

**CLAUDE.md** becomes a lightweight entry point:

- Brief description of SpecSync
- Link to AGENTS.md for full rules
- 20-30 lines instead of 62

**Example CLAUDE.md**:

```markdown
# specsync

SpecSync synchronizes OpenSpec changes with GitHub Issues.

OpenSpec is the planning source of truth. GitHub Issues are the collaboration
projection. Use SpecSync to keep them in sync.

For workflow details, dogfooding rules, security guidelines, and agent principles,
see AGENTS.md.

## Quick Start

1. Create or pull a change: `specsync pull -issue N` or create a change in `openspec/changes/`
2. Plan and implement work
3. Sync to tracker: `specsync -change my-change`
4. See AGENTS.md for detailed workflows

## Commands

specsync is CLI-first. For detailed command guidance, use:

```bash
specsync agent-help <command>
specsync agent-help <command> --json
```

Or read the skill with: `specsync install-skill`
```

**.claude/CLAUDE.md** becomes project-specific project setup:

- Claude Code specific configuration
- Model and capability notes
- References to main AGENTS.md
- 40-50 lines instead of 165

**Example .claude/CLAUDE.md**:

```markdown
# Claude Code Configuration for SpecSync

This project uses OpenSpec and SpecSync for planning and issue sync.

## References

- **Full agent guidelines**: see AGENTS.md
- **Workflow details**: see AGENTS.md
- **SpecSync usage**: see AGENTS.md
- **SpecSync commands**: run `specsync agent-help`

## Key Points

- Plan work in `openspec/changes/` before implementation
- Every commit must reference its issue: `(#N)` or `(closes #N)`
- Always dry-run before syncing: `specsync -change my-change -dry-run`
- Archive completed changes: `openspec archive my-change`

See AGENTS.md for complete rules and dogfooding principles.
```

### 3. Update Content in AGENTS.md

If AGENTS.md is missing details from CLAUDE.md or .claude/CLAUDE.md, merge them in:

- Verify all dogfooding rules are covered
- Verify all workflow details are covered
- Verify all safety principles are covered
- Ensure consistency across sections

### 4. Update Skill

The skill (see `reduce-claude-skill-token-cost`) can now reference:

- AGENTS.md for agent principles
- CLAUDE.md for quick overview
- .claude/CLAUDE.md for Claude Code specifics

## Content Migration

| Current File | Content | Future Location |
|--------------|---------|-----------------|
| AGENTS.md | Principles, rules, workflows | AGENTS.md (authoritative) |
| CLAUDE.md | Problem statement, links | CLAUDE.md (lightweight) |
| .claude/CLAUDE.md | Project setup, references | .claude/CLAUDE.md (lightweight) |

All agent guidance lives in one place (AGENTS.md); others link to it.

## Impact

- **Documentation lines**: 1,197 → ~400 (66% reduction)
- **Maintenance burden**: Reduced; single source of truth
- **Clarity**: Improved; no confusion about which file is authoritative
- **Token cost**: Agents read AGENTS.md once instead of multiple files

## Backwards Compatibility

All files remain readable and accurate. The consolidation is purely organizational:
- CLAUDE.md and .claude/CLAUDE.md become concise entry points
- AGENTS.md is more comprehensive
- Information is not lost; just better organized

## Related Changes

- `reduce-claude-skill-token-cost` — can reference consolidated docs
- No other changes depend on this

## Release Notes

Consolidated agent documentation to reduce duplication. AGENTS.md is now the authoritative agent guide; CLAUDE.md and .claude/CLAUDE.md are lightweight entry points that reference it. No content lost; improved organization and maintainability.
