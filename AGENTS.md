# Agent Workflow

This repo uses OpenSpec as the planning layer and `specsync` as the tracker-sync
layer.

## Principles

- OpenSpec is the source of planning truth.
- Issues are the collaboration surface; `specsync` keeps them aligned with specs.
- Beads is optional and local for personal coordination, not repository data.

## Must / Must Not

- MUST plan work in `openspec/changes/<slug>/` before broad implementation.
- MUST keep `proposal.md` and `tasks.md` accurate while working.
- MUST run `specsync` with `-dry-run` before mutating tracker state.
- MUST add or update tests for code behavior changes.
- MUST NOT commit `.beads/` artifacts.
- MUST NOT commit local `.specsync/` caches from change folders.

## Working Paths

- Spec-first path:
  1. Create/update change in `openspec/changes/<slug>/`.
  2. `specsync -dry-run -slug <slug>`.
  3. `specsync -slug <slug>`.

- Issue-first path:
  1. `specsync pull -issue <n> [-slug <slug>]`.
  2. Refine `proposal.md` and `tasks.md`.
  3. `specsync -dry-run -slug <slug>` then `specsync -slug <slug>`.

## Completion Rule

- When work is complete, ensure tasks are checked, sync once more, and archive
  the completed OpenSpec change.

## Documentation Style

- Keep docs concise and practical.
- Avoid AI-bloated wording and repetition.
- Prefer direct instructions and concrete examples.
