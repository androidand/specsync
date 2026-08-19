---
name: specsync
description: Plan and synchronize OpenSpec changes with GitHub Issues using the specsync CLI. Use when asked to create, update, or reconcile an OpenSpec change with a tracker issue, pull an issue into a local change, scan for related work, cross-link changes, or inspect release impact.
---

# SpecSync

OpenSpec files are the source of planning truth. GitHub Issues are the collaboration projection. Always dry-run before writing.

specsync handles **tracker sync** (OpenSpec ↔ GitHub/Beads).

## Basic Workflow

1. **Create or pull**: `specsync pull -issue N` or create a change in `openspec/changes/<slug>/`
2. **Implement**: Write code, update `tasks.md`
3. **Sync**: `specsync -dry-run -change <slug>` then `specsync -change <slug>`
4. **Complete**: All tasks checked → `specsync` → `openspec archive <slug>`

## Core Commands

**Sync changes to GitHub:**
```bash
specsync -dry-run -change <slug>   # preview
specsync -change <slug>             # push to GitHub
```

**Pull issues into specs:**
```bash
specsync pull -issue N [-change <slug>]
```

**Find related work:**
```bash
specsync scan <path...> [topic]
specsync changes --json             # list all
```

**Link related changes:**
```bash
specsync link <change1> <change2>
```

**Mint a coordination epic and wire its children:**
```bash
specsync epic "Feature X" --repo owner/planning --child owner/backend#12 --child frontend-slug
```

**Get help on any command:**
```bash
specsync agent-help <command>      # read documentation
specsync agent-help <command> --json  # machine-readable
specsync agent-help                # overview
```

## Key Safety Rules

- **Always dry-run first**: `-dry-run` shows what will happen without writing
- **Always pass `-change`**: Without it, syncs every change (usually not what you want)
- **Confirm git remote**: The repo target is detected from `git remote` or `-repo owner/name`

## More Information

For detailed guidance on any command, use `specsync agent-help`. This CLI is self-documenting.

For workflow patterns, see `AGENTS.md` in the repository.

For reference material (command flags, detailed examples, edge cases), see the optional reference files that may be installed alongside this skill.

## Install

```bash
specsync install-skill --claude-code              # install for Claude Code
specsync install-skill --claude-code --profile docs  # with reference docs
specsync install-skill --claude-code --profile full  # legacy (all docs)
```

## Token Efficiency

This skill is optimized for token efficiency. Detailed documentation is available on-demand via `specsync agent-help` rather than loaded by default.

- **Default**: ~280 tokens (minimal profile)
- **With references**: ~450 tokens (docs profile)
- **Legacy**: ~700 tokens (full profile)

For diagnostics: `specsync doctor`
