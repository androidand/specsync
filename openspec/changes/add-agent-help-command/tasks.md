# Tasks

## Design & Implementation

- [ ] Define `AgentCommandHelp` struct
  - [ ] Description (string)
  - [ ] Mutates (bool)
  - [ ] Workflow position and related commands
  - [ ] Flag definitions (name, type, required, default, description)
  - [ ] Safety rules ([]string)
  - [ ] JSON output schema
  - [ ] Examples ([]string)

- [ ] Create command metadata registry
  - [ ] Design the data structure
  - [ ] Add metadata for all 20 subcommands
  - [ ] Ensure accuracy of descriptions and flags
  - [ ] Verify safety rules are correct
  - [ ] Test related-commands references

- [ ] Implement `cmd/specsync/agent_help.go`
  - [ ] Parse command-line arguments
  - [ ] Support `--json` flag
  - [ ] Look up command metadata from registry
  - [ ] Handle "no command specified" (show high-level guidance)
  - [ ] Handle unknown commands (error with suggestions)
  - [ ] Render human-readable output (markdown)
  - [ ] Render JSON output

- [ ] Integrate with main.go
  - [ ] Add "agent-help" to knownSubcommands
  - [ ] Add case for "agent-help" in main switch
  - [ ] Update help text to mention agent-help

## Testing

- [ ] Unit tests for metadata lookup
- [ ] Unit tests for markdown rendering
- [ ] Unit tests for JSON rendering
- [ ] Test all 20 commands have metadata
- [ ] Test --json flag produces valid JSON
- [ ] Test "unknown command" error handling
- [ ] Test output completeness (verify no fields are missing)
- [ ] Integration test: specsync agent-help sync
- [ ] Integration test: specsync agent-help sync --json
- [ ] Verify output matches expected schema

## Documentation

- [ ] Update README.md if needed
- [ ] Document metadata structure in code comments
- [ ] Add examples to cmd/specsync comments

## Validation

- [ ] `make test` passes
- [ ] `make vet` passes
- [ ] `specsync agent-help` works
- [ ] All command help is discoverable
- [ ] CI passes
