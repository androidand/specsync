# Auto-detect Beads provider

Today, using Beads requires explicit `-provider beads`. When `bd` is on PATH or `.beads/` exists, specsync could auto-select the Beads provider without the flag.

## Out of scope

- Multi-provider auto-detection (e.g., choosing between GitHub and Beads when both are available) — that is `multi-provider-sync`
