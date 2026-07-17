# Design: title hygiene on pull

## The problem

`shortenTitle()` already exists and is applied on the sync (outward) path. But `pull` (inward) copies the issue title verbatim into `proposal.md` H1. The dirty title is on disk before cleaning ever happens.

## The fix

Apply `shortenTitle()` in `Pull()` before calling `splitBody()`. The cleaned title becomes the proposal H1. The original title is preserved in `PullResult.TitleBefore` for visibility.

## Why not just fix the agent?

Agent discipline (the skill file convention "Title = WHAT, not HOW") is necessary but insufficient. Agents use multiple tools — `gh`, backlog MCP, specsync — and each has its own entry point. Cleaning on pull catches messy titles regardless of source, at the exact moment they enter the OpenSpec change.

## Output

When the title is cleaned, the CLI prints:
```
title cleaned: "Design: resource-select flavor of the integration fields schema (credential → list resources → multi-create)" -> "Design: resource-select flavor of the integration fields schema"
```

This is visible in both dry-run and real mode, so the user can review before committing.

## Boundary

- `shortenTitle()` is idempotent — if the title is already clean, it returns unchanged. No double-cleaning on subsequent syncs.
- The full original title remains in the issue body for documentation.
- The slug is derived from the original issue title (unchanged behavior).
