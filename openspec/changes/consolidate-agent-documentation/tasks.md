# Tasks

## Analysis

- [ ] Review AGENTS.md, CLAUDE.md, .claude/CLAUDE.md
- [ ] Identify unique content in each file
- [ ] Identify duplicated content across files
- [ ] Create content map: what goes where

## Consolidation

- [ ] Update AGENTS.md to be authoritative
  - [ ] Ensure all dogfooding rules are included
  - [ ] Ensure all workflow patterns are included
  - [ ] Ensure all security guidelines are included
  - [ ] Merge any missing content from other files
  - [ ] Verify no information loss

- [ ] Rewrite CLAUDE.md to be lightweight
  - [ ] Brief description (2-3 lines)
  - [ ] Quick start (3-5 lines)
  - [ ] Link to AGENTS.md for details
  - [ ] Reduce from 62 lines to ~30 lines

- [ ] Rewrite .claude/CLAUDE.md to be lightweight
  - [ ] Project setup and Claude Code specifics
  - [ ] References to AGENTS.md
  - [ ] Reduce from 165 lines to ~50 lines

## Verification

- [ ] Verify all content from original files is preserved
- [ ] Verify AGENTS.md is self-contained and complete
- [ ] Verify CLAUDE.md and .claude/CLAUDE.md properly reference AGENTS.md
- [ ] Verify no contradiction between files
- [ ] Test that agents can follow the new structure

## Documentation

- [ ] Update README if it references these files
- [ ] Ensure links between files work correctly

## Validation

- [ ] All files are readable and coherent
- [ ] No information loss from consolidation
- [ ] No regressions in agent workflows
- [ ] CI passes
