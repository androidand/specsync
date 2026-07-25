# Worktree support: `--worktree` flag for `pull` and `sync`

## Why

Agents working on issues start by reading AGENTS.md conventions, manually creating
a worktree (`git worktree add ../worktrees/<repo>-<issue> <branch>`), changing into
it, then running `specsync pull -issue <n>`. That manual step is friction and a
common source of mistakes — agents sometimes skip the worktree and work directly in
the repo root, scattering branches and leaving stale directories behind.

The real problem is that `pull` and `sync` are **git-context-agnostic**: they read
the current directory's `git remote` and operate on whatever repo is under `pwd`.
They don't know or care whether `pwd` is the repo root or a worktree. The agent
has to bridge that gap manually.

This change makes specsync **worktree-aware** by adding a `--worktree` flag to
`pull` (and optionally `sync`) that creates (or reuses) a worktree, checks out
the feature branch, and runs the operation inside it — all in one command.

## What Changes

- Add a **`--worktree`** flag to `specsync pull` that:
  1. Resolves the target repo from `git remote` (or `-repo` override).
  2. Creates a worktree in a configurable directory (`--worktree-dir`, default
     `../worktrees` or `$SPECSYNC_WORKTREE_DIR`).
  3. Names the worktree `<repo>-<issue>` (e.g. `FusionHub-3538`).
  4. Creates or checks out a feature branch (`feat/<issue>-<slug>`).
  5. `cd`s into the worktree and runs `pull` there.
- The worktree path is **configurable**, not hardcoded — agents follow their
  org's AGENTS.md convention by passing the right `--worktree-dir`.
- `--dry-run` prints the worktree commands without executing them.
- Reuses an existing worktree if one already exists for the same issue (idempotent).
- Does **not** create a PR — that is still the agent's job (via `gh` CLI or the
  skill workflow).

### Out of scope / explicitly deferred
- Automatic PR creation — the agent (guided by the skill) handles `gh pr create`
- Worktree cleanup — stale worktree detection is an agent reasoning pattern, not
  a specsync concern
- Branch naming conventions beyond `<repo>-<issue>` — the agent sets the branch
  name via AGENTS.md; specsync just needs a unique name
- Git worktree removal — `git worktree remove` is a simple CLI call the agent
  makes when done

## Capabilities

### New Capabilities
- `worktree` — a `--worktree` flag for `pull` that creates/reuses a worktree,
  checks out a feature branch, and runs the operation inside it.

## Impact

- New code (Go, stdlib-only): a `--worktree` flag in the `pull` subcommand,
  plus a small worktree manager helper. Consumes the existing `git` shell-out
  infrastructure.
- The specsync skill file gains a one-liner: `specsync pull -issue <n> --worktree`
  replaces the three-step manual sequence.
- The backlog MCP skill (Exopen-specific) references this flag in its worktree
  workflow so agents always create worktrees in the right place.
