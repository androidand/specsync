# Show how to resume each piece of work, including its agent session

## Why

After a crash the git worktrees survive, and so do the issues and pull requests.
What is gone is the context: which terminal held which work, and which agent
session was working in it. `specsync status` can say *what* needs attention
(`status-pr-and-worktree-state`) and *where it lives*
(`work-relations-and-overview`). It cannot say **how to pick it up again**:
the worktree path to `cd` into, the last thing noted about it, and the agent
session to resume.

Session information exists, but it lives in agent-specific places that specsync
must not learn about. specsync is tracker-agnostic and agent-agnostic by design:
it only synchronizes, runs invoke-and-exit, owns no daemon and no memory store.
Hard-coding one agent's session directory would break that, and a copy of session
state in specsync would be a second source of truth that drifts.

So the constraint is: surface sessions without owning them.

## What Changes

- **Session provider hook.** `-sessions-cmd <command>` or `SPECSYNC_SESSIONS_CMD`
  names an external command. At invocation specsync runs it once, reads a JSON
  array from stdout, and joins entries to rows by working directory (longest path
  prefix). Each entry may carry `cwd`, `id`, `name`, `status`, `updatedAt` and an
  opaque `resume` string.
- **Rows gain `sessions` and `resume`.** `sessions` lists matching entries.
  `resume` is a one-line hint: the provider's `resume` string when present,
  otherwise `cd <worktree>`. specsync never interprets or constructs agent
  commands itself.
- **Last note.** Rows for changes also carry `lastNote`: the last line of the
  change's `discoveries.md`, which `specsync note` already writes. This is
  specsync-native, adds no storage, and answers "where did I leave off".
- **Failure is soft.** A missing, slow (timeout, default 5 s) or malformed provider
  adds a status line and leaves rows unchanged. Nothing about sessions can make the
  command fail.
- **An example adapter** ships in the docs for one agent, clearly marked optional.
  It is a script that prints the JSON contract; it is not part of the core.

## Generality

- **Agent-neutral.** specsync knows no agent's file layout; the provider is any command
  that prints the JSON contract, so any agent harness, or none, can supply it.
- **Tracker-neutral and layout-neutral.** Sessions attach by working directory, and the
  last note comes from specsync's own change files.

## Non-goals

- specsync does not store, cache, copy or rank sessions, and has no knowledge of any
  agent's file layout.
- It does not start, resume or kill sessions. `resume` is text for a human or agent
  to run.
- No session history or transcripts, and nothing about their content.
- No claims or presence tracking. That is `in-flight-work-claims` and the
  agent-presence layer in the agent tooling. This only reads what a provider says
  at the moment of the call.

## Security

The provider command is taken only from the flag or the environment of the person
running specsync, never from a file inside a repository. A cloned repository
therefore cannot cause specsync to execute anything. This is the same trust model
as git hooks configured by the user.

## Impact

- New flag `-sessions-cmd`, new env `SPECSYNC_SESSIONS_CMD`, new optional row fields
  `sessions`, `resume`, `lastNote`.
- A small provider runner with timeout and JSON validation; a `discoveries.md`
  reader.
- Docs: README, `agent-help`, the skill, an example adapter under docs, and
  `site/features.json`.

## Release note

`specsync status` can now show how to resume each piece of work: the worktree to
open, the last note on the change, and, through an optional external provider, the
agent session that was working there.
