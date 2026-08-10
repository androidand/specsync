# Tasks

## Implementation

- [ ] Rewrite `skills/specsync/SKILL.md` as minimal router (~90 lines)
  - [ ] Keep high-level purpose and philosophy
  - [ ] Keep single recommended workflow path
  - [ ] Add references to `specsync agent-help`
  - [ ] Keep safety rules (high-level only)
  - [ ] Keep installation instructions
  - [ ] Remove detailed command reference
  - [ ] Remove detailed workflow examples
  - [ ] Remove detailed safety edge cases

- [ ] Create reference documentation structure
  - [ ] Create `skills/specsync/docs/` directory
  - [ ] Move command reference to `docs/reference.md`
  - [ ] Move detailed workflows to `docs/workflow.md`
  - [ ] Move detailed safety rules to `docs/safety.md`
  - [ ] Create `docs/troubleshooting.md` for common issues

- [ ] Update Makefile
  - [ ] Verify `make sync-skill` still works
  - [ ] Ensure embedded SKILL.md in cmd/specsync/ is current
  - [ ] Test build process

## Testing

- [ ] Verify SKILL.md renders correctly in Claude Code
- [ ] Verify all referenced commands exist and work
- [ ] Test that `specsync agent-help` is discoverable from skill
- [ ] Verify backwards compatibility (old installed skill still works)
- [ ] Line count verification: SKILL.md < 100 lines

## Documentation

- [ ] Update README.md if it references SKILL.md
- [ ] Update installation instructions
- [ ] Document reference file location and access
- [ ] Add note about token savings to release notes

## Validation

- [ ] `make test` passes
- [ ] `make vet` passes
- [ ] `specsync validate` passes
- [ ] CI dogfooding gates pass
- [ ] No regressions in existing workflows
