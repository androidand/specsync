# Trim low-value feature cards from the landing page

## Why
site/features.json has grown to one card per shipped PR rather than curated
visitor-facing messaging. Several entries describe internal implementation
details or flag-behavior nuances that read as changelog entries rather than
reasons to try the tool:

- "Explicit repo resolution" and "Per-repository board" — resolution
  precedence order (`-repo` flag → `gh repo set-default` → `origin`, etc.):
  reference-doc material, and "Per-repository board" duplicated "Projects
  board sync" right above it.
- "No label noise by default" — a specific PR's behavior change (labels
  opt-in via `-labels`), not a feature a new visitor evaluates on.

## What Changes
- Remove "Explicit repo resolution", "Per-repository board", and "No label
  noise by default".
- Retitle "Collapsed by default" → "A clean issue, not a wall of text" to
  lead with the visible benefit instead of the implementation mechanism.
- Already-shipped features keep full documentation in README.md; this only
  trims the landing page's card list.

## Impact
- Affected: site/features.json only. No code or behavior change.
