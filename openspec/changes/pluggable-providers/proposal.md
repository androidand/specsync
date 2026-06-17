# Pluggable work providers

## Why

specsync currently shells out to `gh` directly. To support self-hosted trackers
and orchestrators that already own issue lifecycle, the projection target must be
an interface, not a hardcoded binary.

## What

Introduce a `WorkProvider` interface (already present: `Name`, `Push`, `Find`)
and ship multiple implementations behind it:

- `github` — the current `gh` CLI provider (default, zero infra).
- `mcp` — an MCP client that delegates issue create/update/link to an external
  work-management MCP server, reusing its repo-relation knowledge.
- `vikunja` / `plane` — self-hosted providers (later).

The provider is selected by flag/config; the core engine is unchanged.

## Scope

- Provider selection flag (`-provider`) and config resolution.
- `provider/mcp` client implementation.
- Capability interfaces (comments, sub-items, custom fields) detected by type
  assertion so a minimal provider need not implement everything.
