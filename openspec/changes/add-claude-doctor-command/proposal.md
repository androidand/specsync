# Add `specsync doctor` command for Claude Code diagnostics

## Context

Users installing SpecSync may not realize their installation is token-inefficient. Common issues:

- Multiple copies of SKILL.md installed
- Stale skill versions (old SKILL.md left behind)
- Installing the `full` profile when `minimal` would do
- Reference files loaded unnecessarily
- Subagent configurations that waste context
- Version mismatches between binary and skill

Without diagnostics, users keep these inefficient setups indefinitely.

## Proposed Changes

### New Subcommand: `specsync doctor`

#### Basic Usage

```bash
# General diagnostics
specsync doctor

# Claude Code specific
specsync doctor claude

# Skill file diagnostics
specsync doctor skill

# Installation diagnostics
specsync doctor install

# Context/token analysis
specsync doctor context

# Machine-readable output
specsync doctor claude --json
```

#### Output Examples

##### `specsync doctor claude`

Human-readable:

```
SpecSync Claude Code Diagnostics

Installation Status
  Location: ~/.claude/skills/specsync/SKILL.md
  Installed: yes
  Version: 0.9.1 (current binary: 0.9.1) ✓

File Analysis
  Skill file size: 12.6 KB (222 lines)
  Profile: full (includes reference docs)
  Installed: yes
  Stale: no

Skill Copies Detected
  1. ~/.claude/skills/specsync/SKILL.md (12.6 KB) — Primary
  2. ~/.codex/skills/specsync/SKILL.md (12.6 KB) — Duplicate
  3. ~/.config/opencode/skills/specsync/SKILL.md (12.6 KB) — Duplicate

⚠️  WARNING: You have 3 identical skill copies installed.
   This is fine for now, but 2 are redundant. To clean up:
   
   specsync doctor clean --all
   specsync install-skill --claude-code

Token Impact
  Default loaded (worst case): ~700 tokens
  Estimated waste (redundancy): ~400 tokens

Recommendations
  1. If you only use Claude Code, remove other skill copies
  2. If you plan to use other agents, keep them, but consider using
     install profiles (see: specsync agent-help install-skill)
  3. Reference docs are available on-demand; consider using
     --profile minimal to reduce default loaded size to ~280 tokens
```

JSON output:

```json
{
  "status": "warning",
  "timestamp": "2026-08-10T12:34:56Z",
  "installation": {
    "primary": "~/.claude/skills/specsync/SKILL.md",
    "installed": true,
    "version": "0.9.1",
    "current_binary_version": "0.9.1",
    "up_to_date": true
  },
  "file_analysis": {
    "size_bytes": 12900,
    "size_kb": 12.6,
    "lines": 222,
    "profile": "full"
  },
  "duplicates": [
    {
      "path": "~/.claude/skills/specsync/SKILL.md",
      "size_kb": 12.6,
      "primary": true
    },
    {
      "path": "~/.codex/skills/specsync/SKILL.md",
      "size_kb": 12.6,
      "primary": false
    },
    {
      "path": "~/.config/opencode/skills/specsync/SKILL.md",
      "size_kb": 12.6,
      "primary": false
    }
  ],
  "token_analysis": {
    "default_loaded": 700,
    "redundancy_waste": 400,
    "reference_docs_waste": 150
  },
  "issues": [
    {
      "severity": "warning",
      "code": "multiple_skill_copies",
      "message": "You have 3 identical skill copies installed"
    }
  ],
  "recommendations": [
    "Consider using --profile minimal to reduce default token load"
  ]
}
```

##### `specsync doctor install`

```
Installation Locations

Claude Code (.claude/)
  Location: ~/.claude/skills/specsync/
  Installed: yes (12.6 KB)
  Version: 0.9.1
  Status: ✓

Codex (.codex/)
  Location: ~/.codex/skills/specsync/
  Installed: yes (12.6 KB)
  Version: 0.9.1
  Status: ✓ (Unused in this setup)

OpenCode (.config/opencode/)
  Location: ~/.config/opencode/skills/specsync/
  Installed: yes (12.6 KB)
  Version: 0.9.1
  Status: ✓ (Unused in this setup)

Binary
  Version: 0.9.1
  Embedded SKILL.md version: 0.9.1
  Status: ✓

Recommended Action
  You have the skill installed to 3 agent directories. If you only use
  Claude Code, you can clean up the other copies:

    rm ~/.codex/skills/specsync/SKILL.md
    rm ~/.config/opencode/skills/specsync/SKILL.md
```

##### `specsync doctor context`

```
SpecSync Token Impact Analysis

Current Installation
  Profile: full (with reference docs)
  Default loaded: ~700 tokens

By Profile
  minimal: ~280 tokens (router only)
  docs:    ~450 tokens (router + reference files available)
  full:    ~700 tokens (all docs loaded)

Ways to Reduce
  1. Switch to --profile minimal (60% reduction)
     specsync install-skill --claude-code --profile minimal

  2. Don't load reference docs unless needed
     The full profile loads reference.md, workflow.md, safety.md
     by default. Switching to docs profile loads them on-demand.

  3. If you're not using Claude Code for SpecSync, remove the skill
     from other agent directories (see: specsync doctor install)

  4. Use JSON output (--json flag) instead of prose
     JSON is more compact and deterministic
     specsync sync --json
     specsync changes --json

Token Savings Potential: Up to 70% (from 700 → 210 tokens per use)
```

### Subcommands

| Command | Purpose |
|---------|---------|
| `doctor` (default) | General status overview |
| `doctor claude` | Claude Code specific diagnostics |
| `doctor skill` | Skill file analysis and sync status |
| `doctor install` | Installation location detection |
| `doctor context` | Token analysis and recommendations |
| `doctor clean` | (optional) Remove stale installations |

### Implementation Details

Create new file: `cmd/specsync/doctor.go`

Implement functions:
- `runDoctor(args []string)` — dispatcher
- `doctorClaude()` — Claude Code diagnostics
- `doctorSkill()` — skill file analysis
- `doctorInstall()` — installation detection
- `doctorContext()` — token analysis

#### Detection Logic

**Skill installation locations**:
- `~/.claude/skills/specsync/SKILL.md` (Claude Code)
- `~/.codex/skills/specsync/SKILL.md` (Codex/Skein)
- `~/.config/opencode/skills/specsync/` (OpenCode)
- `~/.copilot/skills/specsync/` (Copilot)
- `~/.agents/skills/specsync/` (Generic)

**File analysis**:
- Read file, count lines, measure size
- Parse frontmatter to detect profile
- Compare embedded SKILL.md in binary

**Version detection**:
- Extract version from binary (already available)
- Parse version comment from installed SKILL.md
- Compare for staleness

**Duplication detection**:
- MD5 hash installed files
- Report identical copies
- Suggest cleanup

#### Token Estimation

Rough heuristics:
- SKILL.md ~222 lines → ~700 tokens (1 token ≈ 0.3 lines)
- Router ~90 lines → ~280 tokens
- Reference docs ~600 lines → ~1,800 tokens (but optional)
- (These are rough estimates; actual tokens depend on complexity)

### Benefits

1. **Visibility**: Users can see why token usage is high
2. **Actionable**: Specific, reproducible recommendations
3. **Automated**: Agents can read --json output and auto-tune
4. **Educational**: Teaches users about token waste and solutions
5. **Progressive**: Doesn't force changes; suggests opt-in improvements

## Related Changes

- `reduce-claude-skill-token-cost` — doctor command validates the reduction
- `add-skill-install-profiles` — doctor command recommends profiles
- `add-agent-help-command` — references this command in help

## Release Notes

Added `specsync doctor` command for diagnosing and optimizing Claude Code token usage. Run `specsync doctor` to see installation status, detect duplicate skills, and get personalized recommendations. Use `specsync doctor --json` for automation and scripting.
