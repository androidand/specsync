# Pull creates a missing changes folder

## Why

`specsync pull` stops with "no changes/ directory" in a repo that has `openspec/` but no `openspec/changes/`. Pulling an issue is how a first change starts, so the command that creates the first change should not refuse for the folder that holds it.

## What

- `pull` creates `openspec/changes/` when `openspec/` exists and `changes/` does not, and says so on stderr.
- `pull -dry-run` creates nothing.
- A repo with no `openspec/` still fails as before and names `specsync init`.

## Out of scope

- Other commands keep refusing a missing `changes/`; they have nothing to read.
- `pull` does not create `openspec/` itself. That stays `specsync init` or `pull -init`.

## Release note

`specsync pull` now creates `openspec/changes/` when only that folder is missing, instead of failing.
