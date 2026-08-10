# Tasks

## Design

- [ ] Define output structs for each command
  - [ ] SyncResult
  - [ ] PullResult
  - [ ] LinkResult
  - [ ] TraceResult
  - [ ] ValidateResult
  - [ ] VerifyResult
  - [ ] SpinoffResult
  - [ ] Verify release-plan already has JSON

- [ ] Design JSON schemas
  - [ ] Verify schema design matches command behavior
  - [ ] Ensure all important state is captured
  - [ ] Keep schemas minimal (no redundant fields)
  - [ ] Use consistent field naming conventions

## Implementation

- [ ] Add --json flag to runSync()
- [ ] Add --json flag to runPull()
- [ ] Add --json flag to runLink()
- [ ] Add --json flag to runTrace()
- [ ] Add --json flag to runValidate()
- [ ] Add --json flag to runVerify()
- [ ] Add --json flag to runSpinoff()

For each command:
- [ ] Populate result struct during execution
- [ ] Add JSON output at end of run* function
- [ ] Maintain backwards-compatible prose output when --json not used
- [ ] Test dry-run with --json flag

## Testing

For each command (sync, pull, link, trace, validate, verify, spinoff):

- [ ] Unit tests for result struct marshaling
- [ ] Test --json flag is recognized
- [ ] Test JSON output is valid (unmarshals correctly)
- [ ] Test JSON output has expected fields
- [ ] Test JSON output is complete
- [ ] Test dry-run with --json works
- [ ] Test error cases (what happens when command fails with --json?)
- [ ] Integration test with actual data

## Documentation

- [ ] Update cmd/specsync comments to document JSON output
- [ ] Add JSON schemas to agent-help metadata (see add-agent-help-command)
- [ ] Document output structure in README or docs
- [ ] Add examples showing --json usage

## Validation

- [ ] `make test` passes
- [ ] `make vet` passes
- [ ] `specsync <command> --json` produces valid JSON
- [ ] Prose output unchanged when --json not used
- [ ] CI passes
- [ ] No regressions in existing behavior
