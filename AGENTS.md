# Agent Workflow

This repository uses a spec-first planning model with tracker synchronization.

## Core Philosophy

- OpenSpec is the planning layer.
- specsync is the sync layer between OpenSpec changes and tracker issues.
- Beads is optional local coordination, not repository data.

## Required Tools

Agents working in this repo should use:

- OpenSpec for planning and change lifecycle
- specsync for issue projection and issue-first pull flows
- Beads for local task coordination when useful

## Data Boundaries

- Keep Beads local-only. Do not commit `.beads/` artifacts.
- Do not commit local `.specsync/` caches from change folders.
- Commit OpenSpec changes while active in development.
- After completion, keep only the merged OpenSpec result and archive completed changes.

## Working Loop

1. Create or update an OpenSpec change in `openspec/changes/<slug>/`.
2. Keep `proposal.md` and `tasks.md` current as the source of intent and execution.
3. Run specsync in dry-run first, then sync for real.
4. Implement code and tests, checking off OpenSpec tasks as work is completed.
5. Merge completed OpenSpec results and archive the change.

## Writing Style

- Keep docs concise and practical.
- Avoid AI-bloated wording and repetition.
- Prefer direct instructions and concrete examples.
