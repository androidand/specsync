# Tasks: Declarative Repo Policies

## Phase 1: Schema & Core

- [ ] Add `policies` section to `.openspec.yaml` JSON schema
- [ ] Update OpenSpec type definitions to include `Policies` type
- [ ] Add default policy constants (trackChanges=true, trackConfig=false, trackGeneratedSkills=false)
- [ ] Implement policy resolution with inheritance (walk parent dirs, merge configs)
- [ ] Add `--policy` flag to `specsync init` (track-changes, local-only, custom)

## Phase 2: Tool Integration

- [ ] `specsync init` generates policy-aware `.gitignore` template
- [ ] `specsync doctor` validates tracked files against policy
- [ ] `specsync doctor` explains policy violations with fix suggestions
- [ ] `specsync status` displays effective policy

## Phase 3: Documentation & UX

- [ ] Add `--explain-policy` flag to show dev-facing policy guide
- [ ] Update CLI help text for `init` and `doctor` commands
- [ ] Add policy examples to documentation (design-as-code, issue-driven, hybrid)
- [ ] Write migration guide for existing repos adopting policies

## Phase 4: Testing

- [ ] Unit tests: policy resolution with inheritance
- [ ] Unit tests: policy validation logic
- [ ] Integration tests: `init` generates correct `.gitignore`
- [ ] Integration tests: `doctor` detects violations
- [ ] Fixture repos with different policies for manual testing

## Non-blocking / Future

- Enforcement in CI/pre-commit hooks (external tool responsibility)
- Extended policy language (beyond boolean flags)
- Org-wide policy registry / discovery
