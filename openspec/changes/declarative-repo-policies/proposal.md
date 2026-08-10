# Declarative repo policies for tracking specs in git

## Goal

Add a standard way for projects to declare whether openspec artifacts (specs, config, generated skills) should be tracked in git. This enables specsync and other tools to be **policy-aware** and provide org-specific guidance without requiring workarounds or implicit conventions.

## Why

Different organizations have different needs:
- **Design-as-code teams** want specs committed alongside code (full transparency, history in git)
- **Issue-driven teams** want specs tracked separately (issues are source of truth, specs are ephemeral working docs)
- **Mixed approaches** might commit some artifacts but not others

Currently, specsync has no way to declare these policies, so:

1. Each org reinvents the wheel (gitignore rules, onboarding docs, manual enforcement)
2. Tools like `specsync doctor` can't validate against policy
3. New developers don't know what's expected until they break CI
4. No way to communicate policy intent at initialization time

This organization encountered this when specsync auto-generated skill files were accidentally committed, triggering CI rejection. The policy existed (in memory: "don't track openspec/") but wasn't declarative.

## Proposed solution

Add a `policies` section to `.openspec.yaml`:

```yaml
# .openspec.yaml
metadata:
  version: "1.0"

# Declare org/project policy for what gets tracked in git
policies:
  # Track change definitions and specs in version control
  trackChanges: false          # default: true (design-as-code)
  
  # Track openspec CLI config in version control
  trackConfig: false           # default: false (local tool config)
  
  # Track auto-generated skill files in version control
  trackGeneratedSkills: false  # default: false (from global install)
```

Then:

1. **specsync init** uses a template matching the declared policy
   - If `trackChanges: false`, init suggests `.gitignore` patterns
   - If `trackGeneratedSkills: false`, init warns not to commit `skills/`

2. **specsync doctor** validates against policy
   - If `trackChanges: false` and `openspec/changes/` is tracked → warn
   - If `trackConfig: false` and `openspec/config.yaml` is tracked → warn
   - Explains the policy and how to fix it

3. **Documentation** can reference the policy
   - Specs inherit policy from parent `.openspec.yaml` up the tree
   - Policy applies to all downstream tools

## Design considerations

1. **Inheritance**: Policies at project root apply to all changes/subdirs
2. **Defaults**: Conservative defaults (don't track generated/config) match most use cases
3. **Backwards compat**: Missing `policies` section = assume defaults (no breaking change)
4. **Extensibility**: Room for future policies (e.g., `requireApprovals`, `enforceNaming`)
5. **Multi-repo orgs**: A root `.openspec.yaml` in a mono-repo or org can set org-wide defaults

## What this enables

- ✅ Org-specific best practices without code changes to specsync
- ✅ `specsync doctor` can validate and educate instead of guessing
- ✅ Onboarding: "run `specsync init` and it'll set up your repo correctly"
- ✅ Policy-as-code: `.openspec.yaml` becomes a communication tool
- ✅ Flexibility: Some repos track specs, others don't—all valid

## Open questions

- Should policies be enforceable (block commits) or advisory (warnings only)?
- Should there be a `specsync lint` that validates policy compliance?
- How do we handle migration for existing repos with implicit policies?
