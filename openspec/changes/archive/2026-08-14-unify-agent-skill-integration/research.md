# Research Notes — Agent Skill Integration

Verified against official documentation as of August 2026.

## Claude Code

**Source**: https://code.claude.com/docs/en/skills and https://code.claude.com/docs/en/memory

### Persistent instructions
Claude Code reads `CLAUDE.md` (or `./.claude/CLAUDE.md`) at session start. It does **not** read `AGENTS.md` by default.

**Import mechanism**: `CLAUDE.md` supports `@path/to/file` imports. `@AGENTS.md` imports and expands the file into context at launch. This is the canonical way to share instructions between Claude Code and other agents.

```markdown
@AGENTS.md

## Claude Code
...Claude-specific instructions...
```

A symlink (`ln -s AGENTS.md CLAUDE.md`) also works but `@AGENTS.md` is preferred for cross-platform compatibility.

### Skill locations
| Scope | Path |
|-------|------|
| Personal | `~/.claude/skills/<name>/SKILL.md` |
| Project | `.claude/skills/<name>/SKILL.md` |

Skills follow the [Agent Skills](https://agentskills.io) open standard.

### Skill invocation
- Skills use progressive disclosure: metadata (name + description) loaded at startup, full body loaded only when activated.
- The `description` frontmatter field is used for implicit matching.
- Skills can be explicitly invoked with `/skill-name`.
- `disable-model-invocation: true` prevents automatic loading.

### AGENTS.md section from Claude docs
> "Claude Code reads `CLAUDE.md`, not `AGENTS.md`. If your repository already uses `AGENTS.md` for other coding agents, create a `CLAUDE.md` that imports it so both tools read the same instructions without duplicating them."

## OpenAI Codex

**Source**: https://docs.codex.ai/en/codex/build-skills and https://docs.codex.ai/en/codex/agent-configuration/agents-md

### Persistent instructions
Codex reads `AGENTS.md` from the repository root and parent directories. This is the standard mechanism for repository-level instructions.

### Skill locations
| Scope | Path |
|-------|------|
| Repo (CWD) | `$CWD/.agents/skills/` |
| Repo (parent) | `$CWD/../.agents/skills/` |
| Repo (root) | `$REPO_ROOT/.agents/skills/` |
| User | `$HOME/.agents/skills/` |
| Admin | `/etc/codex/skills/` |

Codex scans `.agents/skills` walking up from CWD to repo root. This is the **Agent Skills standard** location.

### `.codex/skills/` status
The Codex docs do **not** list `~/.codex/skills/` as a skill discovery location. The only user-level global path is `~/.agents/skills/`. The current `~/.codex/skills/` path used by specsync is **not canonical** and should be replaced.

### Skill invocation
- Progressive disclosure: name and description loaded at startup (2% of context window budget).
- Implicit invocation based on `description` field.
- Explicit invocation with `/skills` or `$skill-name`.
- `agents/openai.yaml` can set `policy.allow_implicit_invocation: false`.

## OpenCode

**Source**: https://opencode.ai/docs/skills/ and https://opencode.ai/docs/rules/

### Persistent instructions
OpenCode reads `AGENTS.md` from the project root. It also supports `CLAUDE.md` as a Claude Code compatibility fallback.

### Skill locations
| Scope | Path |
|-------|------|
| Project config | `.opencode/skills/<name>/SKILL.md` |
| Global config | `~/.config/opencode/skills/<name>/SKILL.md` |
| Project Claude-compatible | `.claude/skills/<name>/SKILL.md` |
| Global Claude-compatible | `~/.claude/skills/<name>/SKILL.md` |
| Project agent-compatible | `.agents/skills/<name>/SKILL.md` |
| Global agent-compatible | `~/.agents/skills/<name>/SKILL.md` |

OpenCode discovers skills from **all three** conventions: its own native paths, Claude-compatible paths, and Agent Skills standard paths.

### `instructions` config field
OpenCode supports an `instructions` array in `opencode.json` to load additional instruction files. However, `AGENTS.md` is the simpler, more portable mechanism that works across all three harnesses. The `instructions` field is OpenCode-specific and should not be the primary integration mechanism.

### Skill invocation
- Skills loaded via the native `skill` tool.
- Agent sees available skills in `<available_skills>` section.
- Implicit matching based on `description` frontmatter.
- Explicit invocation via `skill({ name: "skill-name" })`.

## Agent Skills Standard (agentskills.io)

**Source**: https://agentskills.io/specification/

### Canonical paths
| Scope | Path |
|-------|------|
| User | `~/.agents/skills/<name>/SKILL.md` |
| Repo | `.agents/skills/<name>/SKILL.md` |

### Frontmatter (standard fields)
- `name` (required, max 64 chars, lowercase alphanumeric + hyphens)
- `description` (required, max 1024 chars)
- `license` (optional, max 500 chars)
- `compatibility` (optional, max 500 chars)
- `metadata` (optional, string-to-string map)
- `allowed-tools` (optional, experimental)

### Progressive disclosure
1. **Discovery**: name + description loaded at startup
2. **Activation**: full SKILL.md loaded when task matches
3. **Execution**: referenced files loaded on demand

## Verified Compatibility Matrix

| Harness | Persistent instructions | Global skill | Repo-local skill |
|---------|------------------------|--------------|------------------|
| Claude Code | `CLAUDE.md` → `@AGENTS.md` | `~/.claude/skills/specsync` | `.claude/skills/specsync` |
| Codex | `AGENTS.md` | `~/.agents/skills/specsync` | `.agents/skills/specsync` |
| OpenCode | `AGENTS.md` | `~/.agents/skills/specsync` | `.agents/skills/specsync` |

Additional notes:
- OpenCode also supports `~/.claude/skills/` and `~/.config/opencode/skills/` but `.agents/skills/` is the cross-harness standard.
- Claude Code does **not** read `.agents/skills/` — it needs its own `~/.claude/skills/` installation.
- `AGENTS.md` is read by Codex and OpenCode natively. Claude Code needs the `@AGENTS.md` import in `CLAUDE.md`.

## Obsolete / Legacy Paths

The following paths are currently used by specsync but are **not** the canonical location for their respective harness:

| Path | Current target | Status |
|------|---------------|--------|
| `~/.codex/skills/specsync/` | Codex | **Obsolete** — Codex uses `~/.agents/skills/` |
| `~/.config/opencode/skills/specsync/` | OpenCode | **Legacy** — OpenCode supports it but `~/.agents/skills/` is the cross-harness standard |
| `~/.copilot/skills/specsync/` | GitHub Copilot | **Unverified** — Copilot support not confirmed in current docs |

## Skill Profiles

The `add-skill-install-profiles` OpenSpec change proposed `minimal`, `docs`, and `full` profiles but was **never implemented**. The current `installskill.go` has no `--profile` flag and writes the same SKILL.md to all targets. The `doctor` command hardcodes token estimates for profiles that don't exist yet.

## Doctor Diagnostics

Current `doctor.go` implements:
- `doctor claude` — checks `~/.claude/skills/specsync/SKILL.md`
- `doctor install` — checks Claude, Codex, OpenCode paths (uses obsolete paths)
- `doctor context` — static token estimates
- `doctor skill` — checks Claude skill file only

Missing:
- No check for `~/.agents/skills/specsync/` (canonical Agent Skills path)
- No check for `CLAUDE.md` → `@AGENTS.md` import
- No detection of legacy `~/.codex/skills/` or `~/.config/opencode/skills/` installations
- No warnings about duplicate or stale installations
