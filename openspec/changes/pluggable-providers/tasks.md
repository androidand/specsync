# Tasks: Pluggable work providers

- [ ] Extract the provider selection into a `-provider` flag (default: github)
- [ ] Define capability interfaces: CommentCapable, SubItemCapable, CustomFieldCapable
- [ ] Implement `provider/mcp` as an MCP client (stdio + HTTP transports)
- [ ] Document the provider contract in the README
- [ ] Add a fake provider for end-to-end tests of the sync engine
