# Tasks: OpenSpec-native workflow guardrails

## 1. Documentation
- [x] 1.1 Add an "OpenSpec workflow" section to README with expected lifecycle steps
- [x] 1.2 Document issue-first (`specsync pull`) and spec-first (`specsync`) paths as equally valid
- [x] 1.3 Document when `.status` should be used and how it maps to stage labels

## 2. CI guardrails
- [ ] 2.1 Add CI step to validate OpenSpec change structure before sync
- [ ] 2.2 Fail fast in CI when required change files are missing or malformed
- [ ] 2.3 Keep existing sync workflow, but make validation an explicit gate

## 3. Runtime behavior boundaries
- [x] 3.1 Keep file-based parsing as baseline behavior (no hard dependency on OpenSpec CLI)
- [x] 3.2 Add optional guidance for using OpenSpec CLI checks locally when available
- [ ] 3.3 Add tests or checks covering malformed change folder handling