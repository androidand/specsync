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

**Task states.** Beyond `[ ]` (todo) and `[x]` (done), tasks.md supports `[~]` (dropped — superseded or removed) and `[>]` (moved — relocated to another change). Dropped and moved tasks are excluded from progress; only todo and done are "live". The issue body shows a "Plan changes" footer with the breakdown: `+3 added · 2 done · 1 dropped · 1 moved`.

**Discoveries.** Capture findings during implementation without scope decisions. `specsync note -change <slug> "<text>"` appends a line to `discoveries.md`, rendered as `## Discoveries` in the issue. On pull, the section is stripped (like `## Tasks`). The original issue body is saved as `original-ask.md` on first pull (write-once, never overwritten) and rendered as `## Original ask`.

**Provider selection.** `-provider` (repeatable: `github`, `beads`) picks the tracker; absent it, specsync auto-detects. Auto-detection chooses `beads` only when the project carries a `.beads/` directory (repo root or working directory) *and* `bd` is on PATH — `bd` merely being installed is not a signal. Otherwise it chooses `github`. When auto-detection lands on a non-github provider, specsync prints the provider and the reason. If a sync ever names a provider you did not expect, stop and pass `-provider github` explicitly rather than letting it create items in the wrong tracker.

**Lifecycle stages.** specsync labels each issue `stage:<stage>`. The stage is derived automatically: `active` while any task is unchecked, `complete` once every task is checked (before archiving), and `archived` once the change moves under `changes/archive/`. A `.status` file in the change folder overrides the derived stage. This means finishing the last task flips the issue out of `stage:active` on the next sync — no manual bookkeeping.

### Automatic issue resolution (Linker)

When no cached ref exists (no prior sync), specsync can resolve the linked issue
automatically using the **Linker** — a chain of resolvers tried in order until
one hits:

1. **Branch-name resolver** — reads the current branch name and matches a pattern
   like `feat/(\d+)-.*` → issue `#1`. Works for both `sync` and `pull`.
2. **Marker resolver** — parses `<!-- specsync:change=<slug> -->` from the
   provider's issue body (via `Find`).
3. **Cache resolver** — reads `.specsync/refs.json` (existing behavior).
4. **External resolver** — optional hook for MCP or other relation sources.

For `pull`, this means you can omit `-issue` when on a properly-named branch:

```
git checkout -b feat/42-my-feature
specsync pull          # auto-resolves issue #42 from branch name
```

For `sync`, the Linker resolves the issue on first sync without needing a prior
`pull` to cache the ref.

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
specsync link [-dry-run] [-openspec <dir>] <change1> <change2> [<change3>...]
```

At least 2 changes required. Arguments may be **change slugs** (as today) or **issue references** (`#N`, `owner/repo#N`, full URL) — slugs and references can be mixed in one invocation.

- **Slug arguments**: record the link in `links.md` in each change directory, then sync each spec so the "## Related" section appears in each GitHub issue.
- **Reference arguments**: link directly on GitHub with no local spec. specsync fetches the issue, upserts a managed `## Related` section pointing at the other linked issues, and pushes the edited body. Cross-repo targeting is per reference — each `owner/repo#N` resolves to its own repo.

**`links.md` is append-only.** It is yours to write in — prose, dependency order, sequencing notes, `## Blocked by` / `## Blocks` sections — and specsync only ever *adds* entries it does not already find there. Nothing you wrote is rewritten or dropped by `link`, `pull`, or `spinoff`, and a link already recorded (in any spelling: full URL, `owner/repo#N`, or a sibling slug) writes nothing at all. Removing a link is your edit to make.

### Spin off emergent work

```
specsync spinoff -from <slug> -task <n> [-kind bug|followup|task] [-repo owner/name] [-change <slug>] [-dry-run]
specsync spinoff -from <slug> -text "<discovery>" [-kind bug|followup|task] [-repo owner/name] [-change <slug>] [-dry-run]
```

Spawns a new linked change from a discovery, keeping the parent scoped. `-task <n>` extracts text from the nth task line in the parent's `tasks.md` and marks it as moved (`[>] moved: <child-slug>`). `-text` provides free-form discovery text. `-kind` sets an issue label on the child. `-repo` for cross-repo spawn. `-change` overrides the auto-derived slug. The child `proposal.md` is seeded with the discovery text and a provenance line linking to the parent.

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
