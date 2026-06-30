---
name: specsync
description: Plan and synchronize OpenSpec changes with GitHub Issues using the specsync CLI. Use when asked to create, update, or reconcile an OpenSpec change with a tracker issue, pull an issue into a local change, scan for related work, cross-link changes, or inspect release impact.
---

# SpecSync

OpenSpec files are the source of planning truth. GitHub Issues are the collaboration projection. Always dry-run before writing.

## Prerequisites

- **OpenSpec CLI** — install: `npm install -g @fission-ai/openspec@latest`. Initialize a project with `openspec init --tools <tools>` (e.g. `--tools opencode`).
- **GitHub CLI** — needed for `gh issue list` and specsync's GitHub operations.

## Command reference

### Sync a change to GitHub Issues

```
specsync [-dry-run] [-slug <slug>] [-repo owner/name] [-reconcile=false] [-openspec <dir>]
```

- Without `-slug`: syncs **every** change in `openspec/changes/`. Always pass `-slug` when one change is in scope.
- `-dry-run`: prints the `gh` commands and rendered issue body; makes no GitHub calls. Reconcile reads are also skipped in dry-run mode.
- `-reconcile` (default true): on a real sync, reads the issue's checkbox state and writes it back into `tasks.md` before pushing. Pass `-reconcile=false` only to force a one-way projection.
- `-repo owner/name`: override auto-detected repo from `git remote`.

### Pull an issue into a local change

```
specsync pull -issue <n> [-slug <slug>] [-dry-run] [-repo owner/name] [-openspec <dir>]
```

`-issue` is required. Creates `openspec/changes/<slug>/proposal.md` (and `tasks.md` if tasks are detected). `-dry-run` shows what would be written without touching disk.

### Scan for existing work in an area

```
specsync scan [-json] [-openspec <dir>] <path...> [topic words]
```

**At least one path or topic word is required.** Zero-argument scan fails with an error.

Flags (`-json`, `-openspec`) MUST come **before** positional arguments — standard Go flag parsing stops at the first non-flag arg.

Positional args are split automatically:
- **Path**: contains `/`, `*`, `?`, `[`; starts with `.`; or names an existing file/directory.
- **Topic**: all other words are joined into a search topic.

```sh
specsync scan -json cmd/specsync/ "label creation"
specsync scan openspec/changes/ reconcile
specsync scan github.go
```

`-json` emits machine-readable output for planning agents.

### Cross-link two or more changes

```
specsync link [-dry-run] [-openspec <dir>] <slug1> <slug2> [slug3...]
```

At least 2 slugs required. Writes `links.md` into each change directory and syncs them so a "## Related" section appears in each GitHub issue.

### Inspect release impact

```
specsync release-plan [-json] [-since <ref>] [-until <ref>] [-apply] [-openspec <dir>]
```

Read-only follow-up report: shipped changes, gaps, advisory semver bump. `-apply` is advisory only — prints `openspec archive <slug>` commands but does not execute them.

### Install skill globally

```
specsync install-skill [--all] [--claude-code] [--codex] [--opencode] [--copilot]
```

Writes this skill file into the known global agent dirs. `--all` covers every supported platform. Skips dirs that don't exist on the machine.

### Raw trace graph (debugging)

```
specsync trace [-change <slug>] [-since <ref>] [-until <ref>] [-json] [-openspec <dir>]
```

## OpenSpec CLI commands

specsync handles tracker sync. The `openspec` CLI manages the local change lifecycle.

### List changes

```
openspec list [--json]
```

Shows all changes with task completion. `--json` for programmatic use.

### Show a change

```
openspec show <slug>
```

Displays the change's proposal and status.

### Archive a completed change

```
openspec archive <slug> [-y] [--skip-specs]
```

Moves the change to `openspec/changes/archive/` and updates main specs. Use `-y` to skip confirmation.

## Workflow

### Spec-first (plan → issue)

1. `openspec list` — see existing changes.
2. `specsync scan -json <path...> [topic]` — confirm no duplicate change exists.
3. Create or refine `openspec/changes/<slug>/` with at least `proposal.md` and `tasks.md`.
4. `specsync -dry-run -slug <slug>` — inspect the inferred title, body, labels, and checklist.
5. `specsync -slug <slug>` — only when tracker mutation is authorized.

### Issue-first (issue → spec)

1. `gh issue list --state open` — find an issue to work on.
2. `specsync pull -issue <n> -dry-run [-slug <slug>]` — preview generated files.
3. `specsync pull -issue <n> [-slug <slug>]` — write files locally.
4. Refine `proposal.md` and `tasks.md`.
5. `specsync -dry-run -slug <slug>` then `specsync -slug <slug>`.

### Complete a change

1. Ensure all tasks are checked in `tasks.md`.
2. `specsync -slug <slug>` — final sync.
3. `openspec archive <slug> -y` — move to completed.

## Safety rules

- **Always dry-run before any GitHub write.**
- **Always pass `-slug` when one change is in scope.** Omitting it syncs every change.
- Confirm `git remote` resolves to the right repo, or pass `-repo owner/name` explicitly.
- Do not commit `.specsync/` cache directories.
- Never put credentials or sensitive data in issue bodies.
