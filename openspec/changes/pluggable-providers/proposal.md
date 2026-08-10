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

## Release note

Added `-provider mcp`: specsync can now project changes through any external
MCP server instead of `gh` — configured via a committed `.specsync-mcp.json`,
with optional reuse of an existing `.mcp.json` server entry. Speaks the
current MCP spec (2026-07-28, stateless) with automatic fallback to
legacy (pre-2026-07-28, handshake-based) servers. Validated end-to-end
against the real, official `github-mcp-server` (create, rediscover, update),
which also surfaced and fixed a duplicate-creation race shared by every
provider: a short bounded retry now backs off `Find` before concluding an
item doesn't exist, since the local ref cache is deliberately never
committed and a tracker's search index can lag moments behind a very recent
create.
