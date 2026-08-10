# Design: Declarative Repo Policies

## Overview

Extend `.openspec.yaml` schema to include a `policies` section that declares whether openspec artifacts should be tracked in version control. This makes policy explicit and enables tool automation around it.

## Schema addition

```yaml
# In .openspec.yaml or any parent config
policies:
  trackChanges: boolean        # Track openspec/changes/ in git (default: true)
  trackConfig: boolean         # Track openspec/config.yaml in git (default: false)
  trackGeneratedSkills: boolean # Track auto-generated skill files (default: false)
```

### Defaults rationale

| Policy | Default | Reasoning |
|--------|---------|-----------|
| `trackChanges` | `true` | Design-as-code is the primary use case; orgs wanting ephemeral specs opt-out |
| `trackConfig` | `false` | `.openspec.yaml` config is local CLI state, not project config |
| `trackGeneratedSkills` | `false` | Skills come from global installation; duplicating them is waste |

## Implementation areas

### 1. `specsync init`

When initializing a repo, detect or ask about policy:

```bash
specsync init --policy track-changes        # Keep specs in git
specsync init --policy local-only           # Specs stay local (ephemeral)
specsync init --policy custom               # Manual .openspec.yaml setup
```

Generate matching `.gitignore`:

```bash
# If policy.trackChanges = false:
/openspec/changes/
/openspec/.specsync/

# If policy.trackGeneratedSkills = false:
.claude/skills/openspec-*/
.opencode/skills/openspec-*/

# If policy.trackConfig = false:
/openspec/config.yaml
```

### 2. `specsync doctor`

Add policy validation checks:

```bash
$ specsync doctor
Checking policy: trackChanges=false

✓ openspec/changes/ is in .gitignore
✓ No openspec/changes/* files are tracked in git
✓ All change artifacts are local-only
```

If violations found:

```bash
✗ Policy violation: trackChanges=false but openspec/changes/ is tracked in git
  This org keeps design artifacts local. Remove them from git:
    git rm -r openspec/changes/
    git commit -m "chore: remove ephemeral specs from git"
```

### 3. Inheritance

Policies cascade from root down:

```
.openspec.yaml (org-wide policy: trackChanges=false)
└── projects/
    ├── project-a/            # Inherits from parent
    │   └── openspec/changes/ # Subject to org policy
    └── project-b/
        └── .openspec.yaml    # Override parent policy
        └── openspec/changes/ # Uses local policy
```

`specsync status` resolves effective policy:

```bash
$ specsync status
Effective policy (from /org/.openspec.yaml):
  trackChanges: false
  trackConfig: false
  trackGeneratedSkills: false
```

### 4. Documentation generation

`specsync show` or `doctor` can explain policy to new developers:

```bash
$ specsync doctor --explain-policy
This repo's policy (from .openspec.yaml):
  - Design specs are ephemeral (tracked in GitHub issues, not git)
  - openspec/ directory should never be committed
  - Skills come from global specsync installation

To work with specs:
  1. Run: specsync new change "my-feature"
  2. Edit proposal.md, design.md, etc. locally
  3. Run: specsync sync --change my-feature (pushes to issue #NNN)
  4. Never commit openspec/changes/ — it's local-only

Learn more: https://specsync.dev/policies
```

## Not included (defer)

- **Enforcement**: CI/pre-commit hooks that block violations (tool-specific, not specsync's concern)
- **Policy as code**: programmatic validation rules beyond boolean flags
- **Secrets in specs**: orthogonal to this change

## Backwards compatibility

- Missing `policies` section = assume defaults (no breaking change)
- Existing `.openspec.yaml` files work unchanged
- Opt-in: teams adopt by adding `policies` section

## Testing

- Unit: Policy resolution with parent/child configs
- Integration: `specsync init` generates correct `.gitignore`
- Integration: `specsync doctor` detects violations correctly
- Docs: Examples for common policies (design-as-code, issue-driven, hybrid)
