# OpenSpec change validation

CI validates OpenSpec change folder structure before allowing sync.

## Validation

### Requirement: Required files exist

Every change folder must have `proposal.md` and `tasks.md`. Missing files are reported as errors.

### Requirement: Metadata is well-formed

If `.status` or `metadata.json` exists, it must parse correctly. Malformed metadata is reported as an error.

### Requirement: All issues reported in one pass

The validate command reports all structural issues at once, not just the first error.
