# Tasks

## Implementation

- [ ] Create `cmd/specsync/doctor.go`
  - [ ] `runDoctor()` — command dispatcher
  - [ ] `doctorClaude()` — Claude Code diagnostics
  - [ ] `doctorSkill()` — skill file analysis
  - [ ] `doctorInstall()` — installation detection
  - [ ] `doctorContext()` — token analysis

- [ ] Implement skill detection
  - [ ] Search for SKILL.md in known locations
  - [ ] Read file, parse frontmatter
  - [ ] Detect profile from SKILL.md metadata
  - [ ] Count lines and measure size
  - [ ] Detect version from SKILL.md

- [ ] Implement duplication detection
  - [ ] Scan all 5 agent directories
  - [ ] Compare file hashes
  - [ ] Group identical files
  - [ ] Report unique copies vs. duplicates

- [ ] Implement version checking
  - [ ] Extract version from binary (from main.go)
  - [ ] Parse version from installed SKILL.md
  - [ ] Compare for staleness
  - [ ] Report version match/mismatch

- [ ] Implement token analysis
  - [ ] Define token estimation heuristics
  - [ ] Calculate tokens for current installation
  - [ ] Calculate tokens for alternative profiles
  - [ ] Estimate waste from duplicates and unused features

- [ ] Implement recommendations
  - [ ] Generate CLI commands to fix issues
  - [ ] Suggest profile switches
  - [ ] Suggest skill cleanup
  - [ ] Suggest JSON usage

- [ ] Add JSON output support
  - [ ] Define output struct
  - [ ] Implement --json flag
  - [ ] Marshal to JSON
  - [ ] Ensure all data is JSON-serializable

- [ ] Integrate with main.go
  - [ ] Add "doctor" to knownSubcommands
  - [ ] Add case for "doctor" in main switch
  - [ ] Update help text

## Testing

- [ ] Test skill detection (with mock installations)
- [ ] Test duplication detection
- [ ] Test version parsing
- [ ] Test token estimation calculations
- [ ] Test JSON output validity
- [ ] Test on clean system (no skills installed)
- [ ] Test with multiple skill copies
- [ ] Test with stale skill version
- [ ] Verify recommendations are actionable
- [ ] Integration test: specsync doctor
- [ ] Integration test: specsync doctor claude
- [ ] Integration test: specsync doctor --json

## Documentation

- [ ] Add comments to doctor.go
- [ ] Document token estimation methodology
- [ ] Document detection logic
- [ ] Add examples to README or docs

## Validation

- [ ] `make test` passes
- [ ] `make vet` passes
- [ ] `specsync doctor` works
- [ ] `specsync doctor --json` produces valid JSON
- [ ] CI passes
