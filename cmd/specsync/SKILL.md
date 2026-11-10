---
name: specsync
description: Plan and synchronize OpenSpec changes with GitHub Issues using the specsync CLI. Use when asked to create, update, or reconcile an OpenSpec change with a tracker issue, pull an issue into a local change, scan for related work, cross-link changes, inspect release impact, or see where changes stand (issues, branches, worktrees).
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

**Repo has no `openspec/`? That is normal, not a blocker.** Make a scratch one that git never sees, then pull:
```bash
specsync init                       # ./openspec/changes/ + local git exclude; idempotent
specsync pull -issue N -init        # same, then pull (nothing is created without init/-init)
```
Work, `specsync -change <slug>`, and never commit `openspec/`. A shared store works too: `-store <id>`, or `SPECSYNC_DEFAULT_STORE` / `default-store:` in `~/.config/specsync/config.yaml`.

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

**See where your work stands** (use this instead of hand-running `git status`, `git rev-list` and `gh pr view`):
```bash
specsync status                # current worktree: PR, checks, review, unpushed/uncommitted, base moved
specsync status -json          # same facts for agents; read `attention` and `sync`
specsync status -everything    # every change; -state=off for local git facts only
```
`attention` lists what needs a person; `sync` is advice (commands as text) that status never runs. `resume` says how to pick the work up again (`cd <worktree>`, or an agent session when `-sessions-cmd` / `SPECSYNC_SESSIONS_CMD` names a provider) and `lastNote` is the last `specsync note`.

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
- `flow` — Show this repo's flow and mark where your current work is in it.
- `ideas` — List open stage:intake issues for the repo.
- `next` — Say the next step for your current work, and the command for it.
- `overview` — List all work in all repositories, grouped by epic.
- `relate` — Show the work connected to a change or a file path.
- `scan` — Scan for existing work in a code area or topic.
- `status` — Show where your current work stands, or every change with -everything.
- `trace` — Trace dependencies and relationships of a change.

**Plan work** (start: `specsync pull -issue <n>`)
- `adopt` — Bind an existing local change to an existing tracker issue, when neither one knows about the other.
- `epic` — Create a coordination issue and wire children to it.
- `idea` — Capture a free-text idea as a stage:intake issue.
- `init` — Create a scratch openspec/ here and keep it out of git.
- `link` — Cross-link two or more OpenSpec changes.
- `note` — Append a discovery line to a change's discoveries.md.
- `park` — Stop work so another session can resume: save, set the stage and leave a note.
- `pull` — Pull an issue into a local OpenSpec change.
- `save` — Commit your work safely: refuses secrets and forbidden trailers, pushes only when asked.
- `set-priority` — Set or unset a change's numeric priority override.
- `set-stage` — Set the workflow stage of a change.
- `slice` — Start a slice: create its branch and worktree, stacked on the previous slice.
- `slices` — List a change's slices with progress, order, budget and state.
- `spinoff` — Spin off emergent work from a discovery.

**Sync with the tracker** (start: `specsync -dry-run -change <slug>`)
- `sync` — Synchronize an OpenSpec change with GitHub Issues.

**Review and release** (start: `specsync verify`)
- `archive` — Archive a completed change: final push, close, then retention.
- `changelog` — Generate release notes from OpenSpec changes and commits.
- `gates` — Run the repo's configured gates in order and stamp the commit when they pass.
- `lint` — Check titles and issue or PR text against the short-text standard.
- `pr-body` — Generate the correct PR body fragment for a change.
- `release-plan` — Inspect release impact and generate release notes.
- `review-map` — Turn a branch into a review checklist, or prove a rewritten branch has the same content.
- `validate` — Validate OpenSpec change structure.
- `verify` — Verify PR references to their change issues, or with -refs the commit and request references in change documents.

**Housekeeping** (start: `specsync doctor`)
- `agent-help` — Show how to use any specsync command, for people and agents.
- `cleanup` — List finished worktrees and, with -apply, remove them and their branches.
- `doctor` — Diagnose skill installation, environment, and context health.
- `install-skill` — Install the specsync skill for your coding agent.
- `trust` — Allow specsync to run the commands in this repo's specsync.yml.
- `version` — Print the specsync version.
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

This file is the entire installed skill — kept compact so it costs little context; its command list is generated from the binary. Detailed reference material lives in `specsync agent-help` (on-demand, per command) rather than in extra files bundled with the install.

For diagnostics: `specsync doctor`
