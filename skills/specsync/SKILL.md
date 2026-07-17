---
name: specsync
description: Plan and synchronize OpenSpec changes with GitHub Issues using the specsync CLI. Use when asked to create, update, or reconcile an OpenSpec change with a tracker issue, pull an issue into a local change, scan for related work, cross-link changes, or inspect release impact.
---

# SpecSync

OpenSpec files are the source of planning truth. GitHub Issues are the collaboration projection. Always dry-run before writing.

specsync handles **tracker sync** (OpenSpec ↔ GitHub/Beads). Use the **OPSX workflow** (`/opsx:*` commands) for the local change lifecycle: creating, managing, and archiving changes.

## Prerequisites

- **OpenSpec CLI** — install: `npm install -g @fission-ai/openspec@latest`. Initialize with `openspec init --tools <tools>` (e.g. `--tools opencode`).
- **GitHub CLI** — needed for `gh issue list` and specsync's GitHub operations.

## Command reference

### Sync a change to GitHub Issues

```
specsync [-dry-run] [-change <change>] [-repo owner/name] [-reconcile=false] [-close-completed] [-openspec <dir>]
```

- Without `-change`: syncs **every** change in `openspec/changes/`. Always pass `-change` when one change is in scope.
- `-dry-run`: prints the `gh` commands and rendered issue body; makes no GitHub calls. Reconcile reads are also skipped in dry-run mode.
- `-reconcile` (default true): on a real sync, reads the issue's checkbox state and writes it back into `tasks.md` before pushing. The merge is a monotonic union (checked wins), so a lagging issue can never *uncheck* local progress. Pass `-reconcile=false` only to force a one-way projection.
- `-close-completed` (default false): keep tracker open/closed state aligned with completion. Completing every task closes the item; adding new unchecked work reopens it. Without the flag, completion updates `stage:complete` but leaves tracker state alone. An explicit `.status` overrides task-derived stage, so only `.status` value `complete` closes with this flag.
- `-repo owner/name`: override auto-detected repo from `git remote`.

**Lifecycle stages.** specsync labels each issue `stage:<stage>`. The stage is derived automatically: `active` while any task is unchecked, `complete` once every task is checked (before archiving), and `archived` once the change moves under `changes/archive/`. A `.status` file in the change folder overrides the derived stage. This means finishing the last task flips the issue out of `stage:active` on the next sync — no manual bookkeeping.

### Pull an issue into a local change

```
specsync pull -issue <n> [-change <change>] [-dry-run] [-repo owner/name] [-openspec <dir>]
```

`-issue` is required. Creates `openspec/changes/<change>/proposal.md` (and `tasks.md` if tasks are detected). `-dry-run` shows what would be written without touching disk.

**Title hygiene.** specsync never rewrites a title, in either direction: pull writes the issue title verbatim as the proposal H1, and sync pushes the H1 verbatim to the tracker. When a title carries scope detail that belongs in the body (parenthetical asides, backtick markup), both commands print `title could be tighter: "..."` with a suggested variant — edit the proposal H1 yourself if you agree. Write H1s as WHAT, not HOW; put scope in the proposal body.

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

Read-only follow-up report: shipped changes, gaps, advisory semver bump. `-apply` is advisory only — prints `openspec archive <change>` commands but does not execute them.

### Install skill globally

```
specsync install-skill [--all] [--claude-code] [--codex] [--opencode] [--copilot]
```

Writes this skill file into the known global agent dirs. `--all` covers every supported platform. Skips dirs that don't exist on the machine.

### Raw trace graph (debugging)

```
specsync trace [-change <change>] [-since <ref>] [-until <ref>] [-json] [-openspec <dir>]
```

## Workflow

### Spec-first (plan → issue)

1. `/opsx:propose <title>` — create the change with planning artifacts.
2. `specsync scan -json <path...> [topic]` — confirm no duplicate change exists.
3. `specsync -dry-run -change <change>` — inspect the inferred title, body, labels, and checklist.
4. `specsync -change <change>` — only when tracker mutation is authorized.

### Issue-first (issue → spec)

1. `gh issue list --state open` — find an issue to work on.
2. `specsync pull -issue <n> -dry-run [-change <change>]` — preview generated files.
3. `specsync pull -issue <n> [-change <change>]` — write files locally.
4. Refine artifacts with `/opsx:continue` or edit directly.
5. `specsync -dry-run -change <change>` then `specsync -change <change>`.

### Implement

1. `/opsx:apply` — work through tasks, checking them off.
2. `specsync -change <change>` — sync checkbox state to the tracker.

### Complete a change

1. Ensure all tasks are checked in `tasks.md`.
2. `specsync -change <change>` — final sync.
3. `openspec archive <change> -y` — move to completed.

## Safety rules

- **Always dry-run before any GitHub write.**
- **Always pass `-change` when one change is in scope.** Omitting it syncs every change.
- Confirm `git remote` resolves to the right repo, or pass `-repo owner/name` explicitly.
- Do not commit `.specsync/` cache directories.
- Never put credentials or sensitive data in issue bodies.
