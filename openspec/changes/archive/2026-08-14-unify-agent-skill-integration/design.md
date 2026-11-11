# Design — Unify agent skill integration

## Target representation

Current `skillTargets` in `installskill.go`:

```go
var skillTargets = []target{
    {"claude-code", "Claude Code", []string{".claude", "skills", "specsync"}},
    {"codex", "Codex / Skein", []string{".codex", "skills", "specsync"}},
    {"opencode", "OpenCode", []string{".config", "opencode", "skills", "specsync"}},
    {"copilot", "GitHub Copilot", []string{".copilot", "skills", "specsync"}},
    {"agents", "Generic (.agents)", []string{".agents", "skills", "specsync"}},
}
```

New design — two canonical targets plus alias resolution:

```go
var skillTargets = []target{
    {"claude-code", "Claude Code", []string{".claude", "skills", "specsync"}},
    {"agents", "Agent Skills", []string{".agents", "skills", "specsync"}},
}

var aliasTargets = map[string]string{
    "codex":    "agents",
    "opencode": "agents",
}
```

### Alias deduplication logic

When flags are parsed, resolve aliases to their canonical target before iterating:

```go
selected := make(map[string]bool) // canonical flag → selected
for _, t := range skillTargets {
    if *all {
        selected[t.flag] = true
        continue
    }
    if *flags[i] {
        selected[t.flag] = true
        continue
    }
    // Check aliases
    for alias, canonical := range aliasTargets {
        if aliasFlagSet[alias] {
            selected[canonical] = true
        }
    }
}
```

This ensures `--codex --opencode --agents` writes only once to `~/.agents/skills/specsync/`.

### Output UX

```
Installing SpecSync skill...

✓ Claude Code
  ~/.claude/skills/specsync/SKILL.md

✓ Agent Skills
  ~/.agents/skills/specsync/SKILL.md
  Used by: Codex, OpenCode, and other Agent Skills hosts
```

When aliases are used:

```
Installing SpecSync skill...

✓ Agent Skills (also: --codex, --opencode)
  ~/.agents/skills/specsync/SKILL.md
  Used by: Codex, OpenCode, and other Agent Skills hosts
```

## CLAUDE.md rewrite

Current `CLAUDE.md` (37 lines) contains duplicated content from `AGENTS.md` (quick links, TL;DR, repository overview, "where to go next"). All of this is already in `AGENTS.md`.

New `CLAUDE.md` (~12 lines):

```markdown
@AGENTS.md

# SpecSync

This repository uses SpecSync for OpenSpec ↔ issue tracker synchronization.

For SpecSync workflow guidance, use the `specsync` skill.
For command-specific details, prefer `specsync agent-help <command>` over guessing.
```

Key points:
- `@AGENTS.md` imports the full canonical policy
- Two lines of SpecSync-specific guidance
- Explicitly prefers CLI help over guessing
- No duplicated content

## AGENTS.md skill section

Add after "Documentation Style" section:

```markdown
## SpecSync Skill

The `specsync` agent skill is the operational guide for the SpecSync CLI.
Use it whenever planning, creating, pulling, synchronizing, linking,
implementing, completing, or archiving SpecSync/OpenSpec changes.

For command-specific details prefer:

    specsync agent-help <command>

over guessing CLI behavior.
```

## SKILL.md description

Current description (88 chars):

```
Plan and synchronize OpenSpec changes with GitHub Issues using the specsync CLI. Use when asked to create, update, or reconcile an OpenSpec change with a tracker issue, pull an issue into a local change, scan for related work, cross-link changes, or inspect release impact.
```

New description (targeting ~500-600 chars, well within the 1024 limit):

```yaml
description: >
  SpecSync/OpenSpec planning and tracker workflow. Use whenever a repository
  uses SpecSync and the task involves planning work, implementing a planned
  change, creating or updating an OpenSpec change, issues, tasks,
  synchronization, reconciliation, related work, release impact, completion,
  or archiving. Use this skill before guessing SpecSync CLI behavior.
```

### Design decisions on wording

- **No `MUST`**: While the task prompt suggests `MUST`, the Agent Skills spec does not define severity semantics for descriptions. Different harnesses may parse descriptions differently. `Use whenever` and `Use this skill before` are clear directives without overclaiming.
- **Broad scope**: Covers planning, implementing, syncing, completing, archiving — the full lifecycle, not just explicit sync operations.
- **Token efficient**: ~530 characters, well within the 1024 limit. Leaves room for harness-specific additions if needed.
- **Harness portable**: No Claude-specific terminology. Works across all Agent Skills-compatible harnesses.

## Doctor diagnostics design

### New checks

**Canonical path detection:**

```go
canonicalLocations := []struct {
    name   string
    path   string
    agent  string
}{
    {"Claude Code", "~/.claude/skills/specsync/SKILL.md", "claude"},
    {"Agent Skills", "~/.agents/skills/specsync/SKILL.md", "codex,opencode"},
}
```

**Legacy path detection:**

```go
legacyLocations := []string{
    "~/.codex/skills/specsync/SKILL.md",
    "~/.config/opencode/skills/specsync/SKILL.md",
    "~/.copilot/skills/specsync/SKILL.md",
}
```

**CLAUDE.md validation:**

```go
func validateClaudeMD(content []byte) (ok bool, reason string) {
    // Check for @AGENTS.md import
    if bytes.Contains(content, []byte("@AGENTS.md")) {
        return true, "imports AGENTS.md"
    }
    // Check for Markdown link (not an import)
    if bytes.Contains(content, []byte("[AGENTS.md](AGENTS.md)")) {
        return false, "links to AGENTS.md instead of importing it — use @AGENTS.md"
    }
    return false, "does not reference AGENTS.md"
}
```

### Doctor output structure

Human-readable output for `specsync doctor install`:

```
Installation Status

Claude Code
  ~/.claude/skills/specsync/SKILL.md
  Installed: yes (v0.11.2)
  Status: ✓

Agent Skills
  ~/.agents/skills/specsync/SKILL.md
  Installed: no
  Status: ⚠ Not installed
  Run: specsync install-skill --agents

CLAUDE.md
  ./CLAUDE.md
  Imports AGENTS.md: yes
  Status: ✓

Legacy installations (harmless but unnecessary)
  ~/.codex/skills/specsync/SKILL.md — detected, not used by current agents
```

JSON output structure:

```json
{
  "status": "warning",
  "installations": [
    {
      "name": "Claude Code",
      "path": "~/.claude/skills/specsync/SKILL.md",
      "installed": true,
      "version": "0.11.2",
      "canonical": true,
      "status": "ok"
    },
    {
      "name": "Agent Skills",
      "path": "~/.agents/skills/specsync/SKILL.md",
      "installed": false,
      "canonical": true,
      "status": "missing"
    }
  ],
  "legacy": [
    {
      "path": "~/.codex/skills/specsync/SKILL.md",
      "exists": true,
      "note": "Legacy path — Codex uses ~/.agents/skills/"
    }
  ],
  "repository": {
    "agents_md": true,
    "claude_md_imports_agents": true
  },
  "recommendations": [
    "Install Agent Skills skill: specsync install-skill --agents"
  ]
}
```

## Profile compatibility

The `add-skill-install-profiles` change proposed `--profile minimal|docs|full` but was never implemented. This change does not implement profiles — it works with the current single-skill behavior.

When profiles are eventually implemented:
- `--profile` should apply to all selected targets uniformly
- The default profile should be `minimal` (current behavior)
- Profile variants should be generated from `skills/specsync/docs/` at install time
- No changes to target resolution or alias logic

## Sync-skill Makefile target

Current `sync-skill` copies to:
```
skills/specsync/SKILL.md → cmd/specsync/SKILL.md
skills/specsync/SKILL.md → npm/skills/specsync/SKILL.md
skills/specsync/SKILL.md → .claude/skills/specsync/SKILL.md
```

This remains unchanged. The `.claude/skills/specsync/` copy is for testing/verification; the actual Claude Code installation is at `~/.claude/skills/specsync/`.

## Test design

### installskill tests

Use `t.Setenv("HOME", tempDir)` or `os.UserHomeDir` override to avoid modifying the real home directory.

```go
func TestInstallSkillTargets(t *testing.T) {
    // Create temp home directory
    // Set HOME to temp dir
    // Test each flag combination
    // Verify correct files written
    // Verify no duplicate writes
}
```

Test cases:
1. `--claude-code` → writes only to `~/.claude/skills/specsync/SKILL.md`
2. `--agents` → writes only to `~/.agents/skills/specsync/SKILL.md`
3. `--codex` → writes to `~/.agents/skills/specsync/SKILL.md` (alias)
4. `--opencode` → writes to `~/.agents/skills/specsync/SKILL.md` (alias)
5. `--agents --codex` → writes only once to `~/.agents/skills/specsync/SKILL.md`
6. `--codex --opencode` → writes only once to `~/.agents/skills/specsync/SKILL.md`
7. `--all` → writes to both `~/.claude/skills/specsync/SKILL.md` and `~/.agents/skills/specsync/SKILL.md`
8. `--claude-code --agents --codex --opencode --all` → writes to both canonical locations only

### Doctor tests

1. Detects `~/.claude/skills/specsync/SKILL.md` as installed
2. Detects `~/.agents/skills/specsync/SKILL.md` as installed
3. Detects `~/.codex/skills/specsync/SKILL.md` as legacy
4. Detects `~/.config/opencode/skills/specsync/SKILL.md` as legacy
5. Validates `@AGENTS.md` import in CLAUDE.md
6. Flags `[AGENTS.md](AGENTS.md)` as incorrect
7. Reports version status (up to date / needs update)

### CLAUDE.md test

Add a simple validation test that checks `CLAUDE.md` contains `@AGENTS.md`:

```go
func TestClaudeMDImportsAgents(t *testing.T) {
    content, err := os.ReadFile("CLAUDE.md")
    if err != nil {
        t.Fatal(err)
    }
    if !bytes.Contains(content, []byte("@AGENTS.md")) {
        t.Error("CLAUDE.md should import AGENTS.md using @AGENTS.md syntax")
    }
}
```

## Acceptance criteria

1. `CLAUDE.md` uses `@AGENTS.md` import (verified by test)
2. `AGENTS.md` has a "SpecSync Skill" section
3. `install-skill --agents` writes to `~/.agents/skills/specsync/`
4. `install-skill --codex` writes to `~/.agents/skills/specsync/` (alias)
5. `install-skill --opencode` writes to `~/.agents/skills/specsync/` (alias)
6. `install-skill --agents --codex` writes only once
7. `install-skill --all` writes to both canonical locations
8. SKILL.md description covers the full SpecSync lifecycle
9. `specsync doctor` detects canonical and legacy installations
10. `specsync doctor` validates CLAUDE.md → AGENTS.md import
11. All existing tests pass
12. `TestSkillDrift` passes (skill content synchronized)
