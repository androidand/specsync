# Add skill installation profiles for token cost control

## Context

The redesigned SpecSync skill (see `reduce-claude-skill-token-cost`) is much smaller (~90 lines, ~280 tokens) than the original (222 lines, ~700 tokens).

But users have different needs:

- **Minimal users**: Just want the CLI; don't care about embedded docs; save 70% tokens
- **Reference users**: Want optional docs available without loading by default; save 40% tokens
- **Legacy users**: Want the old behavior with all docs loaded; no token savings but zero migration friction

Installation profiles enable user choice.

## Proposed Changes

### Extend `specsync install-skill` with Profiles

#### Basic Usage

```bash
# Install minimal profile (default)
specsync install-skill --claude-code

# Install with optional reference docs
specsync install-skill --claude-code --profile docs

# Install legacy behavior (all docs loaded)
specsync install-skill --claude-code --profile full

# Install multiple agents with a profile
specsync install-skill --all --profile minimal
```

#### Profile Descriptions

| Profile | Size | Loaded | Available | Tokens | Use Case |
|---------|------|--------|-----------|--------|----------|
| **minimal** (default) | ~90 lines | Everything | N/A | ~280 | Users who prefer CLI help or use agent-help |
| **docs** | ~650 lines | ~90 (router) | Ref docs on-demand | ~450 | Users who like reference material but want to save tokens |
| **full** | ~900 lines | Everything | Everything | ~700 | Legacy users, offline work, or users who prefer embedded docs |

### Skill File Organization

```
skills/specsync/
├── SKILL.md                    (Router, ~90 lines)
├── docs/                       (Optional, ~600 lines total)
│   ├── reference.md           (Command reference)
│   ├── workflow.md            (Detailed workflows)
│   ├── safety.md              (Safety rules)
│   └── troubleshooting.md     (Common issues)
└── profiles/
    ├── SKILL.minimal.md       (Router only, ~90 lines)
    ├── SKILL.docs.md          (Router + ref docs, ~650 lines)
    └── SKILL.full.md          (All docs, ~900 lines)
```

Profiles are generated at build/install time:

- `SKILL.minimal.md` — just the router
- `SKILL.docs.md` — router + includes frontmatter directing to doc files
- `SKILL.full.md` — router + embedded reference docs (old behavior)

### Frontmatter Extensions

Each profile declares its scope:

#### SKILL.minimal.md Frontmatter

```yaml
---
name: specsync
description: ... (unchanged)
metadata:
  type: skill
  profile: minimal
  token_estimate: 280
  reference_available: false
  subagents_recommended: false
---
```

#### SKILL.docs.md Frontmatter

```yaml
---
name: specsync
description: ... (unchanged)
metadata:
  type: skill
  profile: docs
  token_estimate: 450
  reference_available: true
  reference_files:
    - docs/reference.md
    - docs/workflow.md
    - docs/safety.md
    - docs/troubleshooting.md
  subagents_recommended: false
---
```

#### SKILL.full.md Frontmatter

```yaml
---
name: specsync
description: ... (unchanged)
metadata:
  type: skill
  profile: full
  token_estimate: 700
  reference_available: true
  subagents_recommended: false
---
```

### Implementation Details

#### Makefile Changes

```makefile
# Generate profile variants
build:
  @go build ./...
  @./scripts/generate-skill-profiles.sh

# Sync profiles to distribution locations
sync-skill:
  # Copy minimal (default) to embedded location
  cp skills/specsync/profiles/SKILL.minimal.md cmd/specsync/SKILL.md
  # Generate npm package with minimal (default)
  cp skills/specsync/profiles/SKILL.minimal.md npm/skills/specsync/SKILL.md
```

#### Skill Generation Script

Create `scripts/generate-skill-profiles.sh`:

```bash
#!/bin/bash
set -e

ROUTER="skills/specsync/SKILL.md"
DOCS_DIR="skills/specsync/docs"
PROFILES_DIR="skills/specsync/profiles"

mkdir -p "$PROFILES_DIR"

# minimal: router only
cp "$ROUTER" "$PROFILES_DIR/SKILL.minimal.md"
# Add minimal frontmatter

# docs: router + reference docs concatenated
cat "$ROUTER" > "$PROFILES_DIR/SKILL.docs.md"
echo "" >> "$PROFILES_DIR/SKILL.docs.md"
echo "## Reference Material (Optional)" >> "$PROFILES_DIR/SKILL.docs.md"
cat "$DOCS_DIR/reference.md" >> "$PROFILES_DIR/SKILL.docs.md"
# ... etc for other docs

# full: legacy (router + all embedded docs)
# ... similar concatenation
```

#### installskill.go Changes

Update `cmd/specsync/installskill.go`:

```go
func runInstallSkill(args []string) {
  fs := flag.NewFlagSet("install-skill", flag.ExitOnError)
  profile := fs.String("profile", "minimal", "minimal, docs, or full")
  // ... existing flags
  
  // Load embedded SKILL.md based on profile
  var skillContent []byte
  switch *profile {
  case "minimal":
    skillContent = embeddedMinimalSkill
  case "docs":
    skillContent = embeddedDocsSkill
  case "full":
    skillContent = embeddedFullSkill
  default:
    fail(fmt.Errorf("unknown profile: %s", *profile))
  }
  
  // Write to all selected directories
  for _, dir := range dirs {
    writeSkillFile(dir, skillContent, *profile)
  }
}
```

#### Embed Multiple Versions

Update cmd/specsync/main.go to embed all three:

```go
import _ "embed"

//go:embed SKILL.md
var embeddedMinimalSkill string

//go:embed SKILL.docs.md
var embeddedDocsSkill string

//go:embed SKILL.full.md
var embeddedFullSkill string
```

Wait, we need to embed the files first. This means we generate all three during build and commit them (or generate at build time).

Alternative: Generate all three at install time from components:

```go
var embeddedRouter string // = SKILL.md content
var embeddedReferenceDocs string // = content of docs/*.md
var embeddedWorkflowDocs string

func generateSkillByProfile(profile string) string {
  switch profile {
  case "minimal":
    return embeddedRouter
  case "docs":
    return embeddedRouter + "\n\n## Reference Material\n\n" + embeddedReferenceDocs
  case "full":
    return embeddedRouter + "\n\n" + embeddedReferenceDocs + "\n\n" + embeddedWorkflowDocs
  }
}
```

### Backwards Compatibility

- Default is `minimal` (significant token savings)
- Existing users can switch to `full` profile if needed
- Installation command remains the same; just add optional `--profile` flag
- No breaking changes

### Migration for Existing Users

Users with the old SKILL.md can:

1. Keep it (works fine, just larger)
2. Upgrade to `minimal`:
   ```bash
   specsync install-skill --claude-code
   ```
3. Upgrade to `docs`:
   ```bash
   specsync install-skill --claude-code --profile docs
   ```
4. Doctor command helps identify which profile is installed and recommend switches:
   ```bash
   specsync doctor claude
   ```

## Token Savings by Profile

| Profile | Default Tokens | Savings vs Full |
|---------|--------|---|
| minimal | ~280 | 60% |
| docs | ~450 | 36% |
| full | ~700 | 0% |

For a developer using SpecSync 5x per week over a year:
- **minimal**: ~72,800 tokens saved/year
- **docs**: ~43,680 tokens saved/year
- **full**: No savings (but backwards compatible)

## Related Changes

- `reduce-claude-skill-token-cost` — provides the router content and docs
- `add-agent-help-command` — minimal profile users rely on this
- `add-claude-doctor-command` — recommends profile switches

## Release Notes

Added installation profiles for SpecSync skill: `minimal` (60% token savings, default), `docs` (36% savings with optional reference files), and `full` (legacy behavior, all docs loaded). Use `specsync install-skill --claude-code --profile <minimal|docs|full>` to choose. Default is `minimal` for optimal token efficiency. Run `specsync doctor` to see which profile is installed and get recommendations.
