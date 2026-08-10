# Tasks

## Design

- [ ] Define profile structure and metadata
  - [ ] Determine what differs between profiles
  - [ ] Design frontmatter extensions
  - [ ] Plan skill file organization
  - [ ] Plan generation strategy

- [ ] Finalize skill content organization
  - [ ] Determine router content (minimal, ~90 lines)
  - [ ] Determine reference docs (docs/, ~600 lines)
  - [ ] Plan full profile (all docs, ~900 lines)
  - [ ] Verify no content loss across profiles

## Implementation

- [ ] Create profile generation script
  - [ ] Create `scripts/generate-skill-profiles.sh`
  - [ ] Generate minimal profile
  - [ ] Generate docs profile (router + ref docs)
  - [ ] Generate full profile (all docs)
  - [ ] Verify output correctness

- [ ] Create profile skill files in skills/specsync/profiles/
  - [ ] SKILL.minimal.md (~90 lines)
  - [ ] SKILL.docs.md (~650 lines)
  - [ ] SKILL.full.md (~900 lines)

- [ ] Create reference documentation in skills/specsync/docs/
  - [ ] reference.md
  - [ ] workflow.md
  - [ ] safety.md
  - [ ] troubleshooting.md

- [ ] Update Makefile
  - [ ] Add profile generation to build target
  - [ ] Update sync-skill to use minimal as default
  - [ ] Verify embedded SKILL.md is minimal version

- [ ] Update cmd/specsync/installskill.go
  - [ ] Add --profile flag
  - [ ] Accept "minimal", "docs", "full" values
  - [ ] Default to "minimal"
  - [ ] Load correct embedded skill variant
  - [ ] Write to agent directories

- [ ] Update cmd/specsync/main.go
  - [ ] Embed all three profile variants
  - [ ] OR: embed components and generate at install time
  - [ ] Verify embeddings work correctly

- [ ] Update npm postinstall
  - [ ] Ensure npm publishes minimal by default
  - [ ] Document how users can install different profiles

## Testing

- [ ] Test profile generation script
- [ ] Test each profile installs correctly
- [ ] Test minimal profile contains only router
- [ ] Test docs profile contains router + reference
- [ ] Test full profile contains all content
- [ ] Test --profile flag is recognized
- [ ] Test default (without --profile) uses minimal
- [ ] Test all three profiles install to correct locations
- [ ] Test backwards compatibility (existing installations still work)
- [ ] Test token estimates are reasonable
- [ ] Integration test: specsync install-skill --profile minimal
- [ ] Integration test: specsync install-skill --profile docs
- [ ] Integration test: specsync install-skill --profile full

## Documentation

- [ ] Document profile usage in README
- [ ] Document profile differences
- [ ] Add examples to install-skill help
- [ ] Document when to use each profile
- [ ] Document migration path from full → minimal

## Validation

- [ ] `make test` passes
- [ ] `make vet` passes
- [ ] `specsync install-skill --profile minimal` works
- [ ] Skill files are valid markdown
- [ ] CI passes
- [ ] npm publishing works with minimal profile
- [ ] No regressions in existing behavior
