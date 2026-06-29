# Multi-Agent Skill Install

## Problem

specsync has no mechanism to distribute itself as an AI agent skill. Agents working in projects that use specsync must either discover AGENTS.md by chance, or rely on a manually installed global skill that may be stale or absent. When an agent creates its own skill from assumptions (as happened in practice), it gets the CLI wrong and causes errors.

The agentskills.io open standard (converged on by Claude Code, Codex, OpenCode, Copilot, Cursor, Gemini CLI, Amp, and others) uses an identical `SKILL.md` format across all platforms — one file, multiple install locations. specsync MUST ship and install this file.

## Context

- Skein's skill directory situation is in flux; another effort is evaluating whether Skein should rely on specsync directly. Skein support here is therefore marked provisional and MUST be easy to update or remove without breaking the rest.
- The existing `npm/scripts/postinstall.js` already handles platform-specific binary install. Skill install follows the same pattern.
- A `specsync install-skill` Go subcommand is novel (no other tool does this) and useful for Homebrew / `go install` paths that bypass npm.

## Requirements

### Canonical skill file

- MUST add `skills/specsync/SKILL.md` as the single source of truth for the specsync skill content.
- The file MUST use the agentskills.io frontmatter schema (`name`, `description`).
- Content MUST match the verified CLI behavior (scan requires path/topic; flags before positional args; -slug always required for single-change sync; etc.).

### In-repo project skill

- MUST add `.claude/skills/specsync/SKILL.md` (copy, not symlink — symlinks don't survive npm pack or git clone on all platforms).
- This gives any agent working in the specsync repo itself immediate access to the skill.

### Copilot always-on context

- MUST add `.github/copilot-instructions.md` with a concise specsync reference.
- This is a separate concept from skills (passive, always injected) and complements the SKILL.md.

### npm postinstall install

- MUST extend `npm/scripts/postinstall.js` to copy `skills/specsync/SKILL.md` into each known global agent skill dir that already exists on the machine:
  - `~/.claude/skills/specsync/`
  - `~/.codex/skills/specsync/`
  - `~/.config/opencode/skills/specsync/`
  - `~/.copilot/skills/specsync/`
  - `~/.agents/skills/specsync/`
- Install MUST be non-fatal (skip dirs that don't exist, log but don't fail).
- Install MUST be idempotent (overwrite if already present, so upgrades self-heal).
- `npm/package.json` MUST add `"skills/"` to the `files` array.

### CLI subcommand: `specsync install-skill`

- MUST add an `install-skill` subcommand to the Go CLI.
- MUST support `--all` (install to every known dir) and individual flags `--claude-code`, `--codex`, `--opencode`, `--copilot`.
- MUST print which dirs were written and which were skipped (not found).
- The skill file content MUST be embedded in the binary (Go `embed`), not read from disk, so it works after `go install` or Homebrew install with no `skills/` directory present.
- Skein support is provisional: add `--skein` flag targeting `~/.codex/skills/` (shared dir) but document it as provisional pending the Skein/specsync compatibility work.

### npm package update

- MUST add `"skills/"` to the `files` array in `npm/package.json`.
- MUST NOT bump the version (no functional CLI change in this PR).
