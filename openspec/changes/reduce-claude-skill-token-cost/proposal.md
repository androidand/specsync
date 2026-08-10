# Reduce Claude Code skill token cost to ~10% of current

## Context

The current `specsync` skill for Claude Code is 222 lines of detailed reference material covering all commands, workflows, and safety rules. This teaching material enters Claude's context every time the skill is invoked, causing significant token waste—especially when the same manual is duplicated in AGENTS.md, CLAUDE.md, and WORKFLOW.md.

The core problem: **Claude learns SpecSync** instead of **Claude asks SpecSync**.

This change redesigns the skill as a minimal router that delegates detailed guidance to:

1. CLI-generated help (new `agent-help` command)
2. Reference documentation (loaded on-demand, not by default)
3. Deterministic JSON output (new `--json` flags on key commands)

## Proposed Changes

### 1. Redesign SKILL.md as a Tiny Router

Reduce from 222 lines to ~90 lines. Focus on:

- High-level purpose and philosophy
- Single recommended workflow (plan → track)
- Invoke deterministic CLI tools (scan, sync, pull)
- Reference to `specsync agent-help` for detailed guidance
- Safety rules (without detailed examples)
- Installation instructions

### 2. Move Reference Material to Structured Docs

Create optional reference files in `skills/specsync/docs/`:

- `reference.md` — full command reference (moved from SKILL.md)
- `workflow.md` — detailed workflow patterns (moved from SKILL.md)
- `safety.md` — detailed safety rules and edge cases (moved from SKILL.md)
- `troubleshooting.md` — common issues and solutions

These files are installed only when using the `docs` or `full` profile (see `add-skill-install-profiles`).

### 3. Update Makefile

The `make sync-skill` target currently copies SKILL.md to multiple locations. This change:

- Keeps the canonical SKILL.md in `skills/specsync/SKILL.md`
- Generates profiles at build/install time (not compile time)
- Stores profile variants for distribution

## Impact on Token Usage

| Metric | Before | After | Savings |
|--------|--------|-------|---------|
| SKILL.md lines | 222 | 90 | 60% |
| Default loaded (lines) | 222 | 90 | 60% |
| Optional (lines) | 0 | 600 | — |
| Estimated token cost | ~700 tokens | ~280 tokens | 60% |

Assumes:
- Typical skill invoke: Claude reads SKILL.md once
- Users running common workflows rarely need detailed reference
- Optional docs prevent knowledge loss (progressive disclosure)

## No Functionality Loss

All existing SpecSync capabilities remain:

- Every command is still available
- Full reference material still exists (in structured docs)
- Safety rules preserved
- Workflow patterns preserved
- Migration path for existing users

## Design Principles

1. **Micro-skill**: Teach the fewest concepts needed to start
2. **Progressive disclosure**: Full reference available on-demand
3. **CLI-first**: Delegate to deterministic tools (scan, sync, pull)
4. **JSON-driven**: Prefer structured output over prose parsing
5. **Backwards compatible**: Minimal install profile is default, full profile available

## Related Changes

- `add-agent-help-command` — enables skill to reference `specsync agent-help`
- `add-json-output-to-key-commands` — enables Claude to consume structured output
- `add-skill-install-profiles` — enables users to choose token cost vs. reference completeness
- `consolidate-agent-documentation` — reduces duplication across AGENTS.md, CLAUDE.md, SKILL.md

## Migration Notes

**For existing users**: Reinstall the skill to get the reduced version.

```bash
specsync install-skill --claude-code
```

If you want the full reference material, use:

```bash
specsync install-skill --claude-code --profile full
```

Or install the reference docs separately:

```bash
specsync install-skill --claude-code --profile docs
```

## Release Notes

Redesigned the SpecSync Claude Code skill for 60% token reduction. The skill is now a minimal router; detailed reference material is available on-demand via `specsync agent-help` and optional reference documentation. No functionality lost.
