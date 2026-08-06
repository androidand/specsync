# Make links.md append-only, never rewritten

## Problem

specsync currently overwrites `links.md` entirely on each run, destroying any
authored content — prose, sequencing notes, and `## Blocked by` / `## Blocks`
sections — that the author added.

## Outcome

`links.md` becomes append-only. New entries are added only when not already
present. Authored content is preserved.
