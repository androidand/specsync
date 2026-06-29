# specsync

specsync projects OpenSpec changes onto GitHub Issues. OpenSpec files are the planning truth; issues are the collaboration surface.

## Key commands

```sh
specsync scan [-json] <path...> [topic]   # find related changes (path/topic required)
specsync -dry-run -slug <slug>            # preview issue without writing
specsync -slug <slug>                     # sync one change to its issue
specsync pull -issue <n>                  # pull an issue into a local change
specsync link <slug1> <slug2>             # cross-link two changes
specsync release-plan [-json]             # advisory semver bump + shipped changes
specsync install-skill [--all]            # install skill to all agent dirs
```

## Rules

- Always `-dry-run` before any GitHub write.
- Always pass `-slug` when working on one change — omitting it syncs every change.
- `scan` flags must come **before** positional args. Zero-arg scan fails.
- Do not commit `.specsync/` cache directories.

## Workflow (spec-first)

1. `specsync scan -json <path> [topic]` — check for existing work
2. Write `openspec/changes/<slug>/proposal.md` and `tasks.md`
3. `specsync -dry-run -slug <slug>` — inspect output
4. `specsync -slug <slug>` — push to GitHub
