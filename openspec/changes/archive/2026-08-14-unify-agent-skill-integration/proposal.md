# Unify agent skill integration

## Problem

A repository using SpecSync currently requires different, partially incorrect setup for each AI coding agent to get consistent behavior:

1. **Claude Code** reads `CLAUDE.md` but the current file uses a Markdown link `[AGENTS.md](AGENTS.md)` instead of the `@AGENTS.md` import syntax. This means Claude Code gets a summary of the agent workflow, not the full canonical instructions from `AGENTS.md`.

2. **Codex** discovers skills from `~/.agents/skills/` (the Agent Skills standard), but specsync's `install-skill` command writes to `~/.codex/skills/` — a path that Codex does **not** scan. Users who run `specsync install-skill --codex` get nothing.

3. **OpenCode** supports `.agents/skills/`, `~/.agents/skills/`, `.claude/skills/`, `~/.claude/skills/`, and its own `~/.config/opencode/skills/`. Specsync currently only installs to the OpenCode-native path, missing the cross-harness standard.

4. **Skill invocation** is unreliable. The current skill `description` only mentions explicit synchronization operations (create, update, reconcile, pull, scan, link, release impact). It does not cover planning, implementing, or general SpecSync workflow questions — the exact situations where implicit invocation matters most.

5. **Doctor diagnostics** only check Claude-specific paths and use obsolete Codex/OpenCode paths. They do not detect the canonical `.agents/skills/` installation, do not validate `CLAUDE.md` → `@AGENTS.md` imports, and do not warn about legacy duplicate installations.

## Context

This repo has already implemented (but not yet archived):
- Skill reduction from 222 → ~80 lines (`reduce-claude-skill-token-cost`)
- `specsync agent-help` for CLI self-documentation (`add-agent-help-command`)
- `specsync doctor` with subcommands (`add-claude-doctor-command`)
- Reference docs in `skills/specsync/docs/` (`reduce-claude-skill-token-cost`)

But `install-skill` still targets obsolete paths, `CLAUDE.md` does not import `AGENTS.md`, and the skill description is too narrow.

## Architecture

The solution uses three layers, preserving SpecSync's existing design philosophy:

```
Layer 1 — Persistent repository policy (always loaded)
    AGENTS.md — canonical for all agents
    CLAUDE.md → @AGENTS.md — Claude Code receives the same policy

Layer 2 — Discoverable agent skill (loaded on demand)
    ~/.agents/skills/specsync/SKILL.md — shared by Codex, OpenCode, others
    ~/.claude/skills/specsync/SKILL.md — Claude Code only

Layer 3 — Deterministic CLI documentation (on request)
    specsync agent-help <command> [ --json ]
```

### Layer 1: Persistent instructions

`AGENTS.md` remains the single source of truth. It already contains the complete SpecSync workflow, rules, dogfooding requirements, and conventions.

Claude Code receives the same content through `@AGENTS.md` import in `CLAUDE.md`. The new `CLAUDE.md` is brief — it imports `AGENTS.md` and adds a short pointer to the SpecSync skill.

OpenCode and Codex read `AGENTS.md` natively. No changes needed.

A new concise section in `AGENTS.md` explains the relationship between repository policy, the skill, and CLI help.

### Layer 2: Agent skill

The skill installs to two canonical locations:

- `~/.claude/skills/specsync/SKILL.md` — Claude Code
- `~/.agents/skills/specsync/SKILL.md` — Codex, OpenCode, and other Agent Skills hosts

The skill `description` frontmatter is broadened to cover the full SpecSync/OpenSpec lifecycle (planning, implementing, syncing, completing, archiving) so implicit invocation is reliable across all harnesses.

### Layer 3: CLI documentation

Unchanged. `specsync agent-help` remains the authoritative source for command-specific details. The skill continues to route agents to it.

## Installation model

```bash
specsync install-skill --all
```

Produces:

```
✓ Claude Code
  ~/.claude/skills/specsync/SKILL.md

✓ Agent Skills
  ~/.agents/skills/specsync/SKILL.md
  Used by: Codex, OpenCode, and other Agent Skills hosts
```

### Compatibility flags

| Flag | Destination | Notes |
|------|------------|-------|
| `--claude-code` | `~/.claude/skills/specsync/` | Claude Code native |
| `--agents` | `~/.agents/skills/specsync/` | Agent Skills standard |
| `--codex` | `~/.agents/skills/specsync/` | Alias for `--agents` |
| `--opencode` | `~/.agents/skills/specsync/` | Alias for `--agents` |
| `--all` | Claude Code + Agent Skills | Installs to both canonical locations |

`--codex` and `--opencode` become compatibility aliases that resolve to the same destination as `--agents`. When both are specified (or combined with `--agents`), only one physical write occurs.

Existing users with `--codex` or `--opencode` flags continue to work — they get the correct modern installation.

## Diagnostics

`specsync doctor` gains awareness of the unified installation model:

- Detects canonical Claude Code installation (`~/.claude/skills/specsync/`)
- Detects canonical Agent Skills installation (`~/.agents/skills/specsync/`)
- Detects legacy `~/.codex/skills/specsync/` and `~/.config/opencode/skills/specsync/` installations
- Validates `CLAUDE.md` imports `AGENTS.md` (not just links to it)
- Reports version mismatches
- Distinguishes errors, warnings, and informational messages

## Skill description

Broadened to cover the full lifecycle:

```yaml
description: >
  SpecSync/OpenSpec planning and tracker workflow. Use whenever a repository
  uses SpecSync and the task involves planning work, implementing a planned
  change, creating or updating an OpenSpec change, issues, tasks,
  synchronization, reconciliation, related work, release impact, completion,
  or archiving. Use this skill before guessing SpecSync CLI behavior.
```

## Backwards compatibility

- `--codex` and `--opencode` flags continue working as aliases
- `--all` installs to the same locations as before (Claude Code + Agent Skills, replacing the old 5-target approach)
- Legacy installations at `~/.codex/skills/` and `~/.config/opencode/skills/` are left untouched but flagged by doctor
- No breaking changes to existing commands or behavior

## Non-goals

- Does not implement skill profiles (proposed in `add-skill-install-profiles`, not yet implemented — out of scope)
- Does not redesign `specsync agent-help` or `specsync doctor` internals
- Does not add repository-local skill installation (relies on `AGENTS.md` for repo behavior)
- Does not delete legacy skill files automatically
- Does not invent harness-specific configuration fields

## Related changes

- `reduce-claude-skill-token-cost` — provides the reduced skill content and reference docs
- `add-agent-help-command` — provides CLI self-documentation
- `add-claude-doctor-command` — provides the doctor subcommand foundation
- `consolidate-agent-documentation` — proposed CLAUDE.md consolidation but was never implemented; this change completes the CLAUDE.md → AGENTS.md import

## Release notes

Unified agent skill installation to follow the Agent Skills standard. `specsync install-skill --all` now installs to `~/.claude/skills/specsync/` (Claude Code) and `~/.agents/skills/specsync/` (Codex, OpenCode, and other Agent Skills hosts). `--codex` and `--opencode` are now aliases for `--agents`. CLAUDE.md imports AGENTS.md so Claude Code receives the same canonical repository policy as other agents. Run `specsync doctor` to check your installation.
