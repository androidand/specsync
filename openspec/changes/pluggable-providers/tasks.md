# Tasks: Pluggable work providers

- [x] Extract the provider selection into a `-provider` flag (default: github)
- [x] Define capability interfaces: CommentCapable, SubItemCapable, CustomFieldCapable
- [x] Implement `provider/mcp` as an MCP client (stdio + HTTP transports)
- [x] Document the provider contract in the README
- [x] Add a fake provider for end-to-end tests of the sync engine
