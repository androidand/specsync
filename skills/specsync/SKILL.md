---
name: specsync
description: Plan and synchronize OpenSpec changes with GitHub Issues using the specsync CLI. Use when asked to create, update, or reconcile an OpenSpec change with a tracker issue, pull an issue into a local change, scan for related work, cross-link changes, or inspect release impact.
---

# SpecSync

OpenSpec files are the source of planning truth. GitHub Issues are the collaboration projection. Always dry-run before writing.

specsync handles **tracker sync** (OpenSpec ↔ GitHub/Beads).

## Basic Workflow

1. **Create or pull**: `specsync pull -issue N` or create a change in `openspec/changes/<slug>/`
2. **Implement**: Write code, update `tasks.md`
3. **Sync**: `specsync -dry-run -change <slug>` then `specsync -change <slug>`
4. **Complete**: All tasks checked → `specsync` → `openspec archive <slug>`

## Core Commands

**Sync changes to GitHub:**
```bash
specsync -dry-run -change <slug>   # preview
specsync -change <slug>             # push to GitHub
```

**Pull issues into specs:**
```bash
specsync pull -issue N [-change <slug>]
specsync pull -issue N -worktree   # also creates/reuses a dedicated git worktree + branch
```
One issue, one branch, one worktree — don't stack unrelated changes on whatever branch is currently checked out. `-worktree` sets both up for the issue-first path; see `specsync agent-help` for the general convention (also applies spec-first, where you branch/worktree by hand: `git worktree add ../worktrees/<slug> -b feat/<n>-<slug>`).

**Bind an already-existing change to an already-existing issue:**
```bash
specsync adopt -issue N [-change <slug>]
```
Use this — never hand-edit the `<!-- specsync:change=<slug> -->` marker into an
issue body yourself. Marker discovery goes through search indexing, which lags
writes by seconds to minutes; a sync inside that window finds nothing and
creates a duplicate issue. `adopt` writes the ref cache first, so the link
holds immediately regardless of indexing.

**Find related work:**
```bash
specsync scan <path...> [topic]
specsync changes --json             # list all
```

**Link related changes:**
```bash
specsync link <change1> <change2>
```

**Mint a coordination epic and wire its children:**
```bash
specsync epic "Feature X" --repo owner/planning --child owner/backend#12 --child frontend-slug
```

**Get help on any command:**
```bash
specsync agent-help <command>      # read documentation
specsync agent-help <command> --json  # machine-readable
specsync agent-help                # overview
```

## All Commands

<!-- specsync:commands:start -->
**See what is going on** (start: `specsync changes`)
- `audit` — Audit OpenSpec changes for structural issues.
- `audit-tasks` — Audit that task status matches code changes (dogfooding).
- `changes` — List all OpenSpec changes with status.
- `ideas` — List open stage:intake issues for the repo.
- `relate` — Show the work connected to a change or a file path.
- `scan` — Scan for existing work in a code area or topic.
- `status` — Show each change with its issue, branch and worktree.
- `trace` — Trace dependencies and relationships of a change.

**Plan work** (start: `specsync pull -issue <n>`)
- `adopt` — Bind an existing local change to an existing tracker issue, when neither one knows about the other.
- `epic` — Create a coordination issue and wire children to it.
- `idea` — Capture a free-text idea as a stage:intake issue.
- `link` — Cross-link two or more OpenSpec changes.
- `note` — Append a discovery line to a change's discoveries.md.
- `pull` — Pull an issue into a local OpenSpec change.
- `set-priority` — Set or unset a change's numeric priority override.
- `set-stage` — Set the workflow stage of a change.
- `spinoff` — Spin off emergent work from a discovery.

**Sync with the tracker** (start: `specsync -dry-run -change <slug>`)
- `sync` — Synchronize an OpenSpec change with GitHub Issues.

**Review and release** (start: `specsync verify`)
- `archive` — Archive a completed change: final push, close, then retention.
- `changelog` — Generate release notes from OpenSpec changes and commits.
- `pr-body` — Generate the correct PR body fragment for a change.
- `release-plan` — Inspect release impact and generate release notes.
- `validate` — Validate OpenSpec change structure.
- `verify` — Verify PR references to their change issues.

**Housekeeping** (start: `specsync doctor`)
- `agent-help` — Show how to use any specsync command, for people and agents.
- `doctor` — Diagnose skill installation, environment, and context health.
- `install-skill` — Install the specsync skill for your coding agent.
<!-- specsync:commands:end -->

## Key Safety Rules

- **Always dry-run first**: `-dry-run` shows what will happen without writing
- **Always pass `-change`**: Without it, syncs every change (usually not what you want)
- **Confirm git remote**: The repo target is detected from `git remote` or `-repo owner/name`

## More Information

For detailed guidance on any command, use `specsync agent-help` — this CLI is self-documenting, so reach for it instead of guessing at flags.

For workflow patterns, see `AGENTS.md` in the repository (if the project has one).

## Install

```bash
specsync install-skill --claude-code   # install for Claude Code
specsync install-skill --all           # every known agent directory: --codex --opencode --copilot --agents
```

This file is the entire installed skill — deliberately small so it costs little context by default. Detailed reference material lives in `specsync agent-help` (on-demand, per command) rather than in extra files bundled with the install.

For diagnostics: `specsync doctor`
