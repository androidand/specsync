# Beads auto-detection

When Beads is available (bd on PATH or .beads/ exists), specsync auto-selects the Beads provider without requiring `-provider beads`.

## Auto-detection

### Requirement: Detect bd on PATH

When `bd` is found on PATH, specsync auto-selects the Beads provider as the default.

### Requirement: Detect .beads/ directory

When `.beads/` exists in the repo root, specsync auto-selects the Beads provider.

### Requirement: Explicit flag overrides

`-provider github` always overrides auto-detection.
