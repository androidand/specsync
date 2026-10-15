# Design

## Provider contract

stdout is a JSON array. Unknown fields are ignored; every field is optional except
`cwd`, which is the join key.

```json
[
  {"cwd": "/path/to/worktree", "id": "s-123", "name": "api-scope",
   "status": "idle", "updatedAt": "2026-10-13T09:12:00Z",
   "resume": "agent --resume s-123"}
]
```

Joining uses the longest path prefix of `cwd`, so a session started in a
subdirectory still attaches to its worktree. Entries that match no row are counted
in a status line and not shown.

## Why an external command

A provider keeps specsync out of any agent's file layout and out of the business of
deciding what a "session" is. It also covers the case the first request named: after
a crash, "live" sessions are gone but transcripts remain, and only the agent knows
how to find and resume the latest one. specsync prints the provider's answer and
nothing more.

Alternative considered: read one agent's session directory directly. Rejected: it
couples a tracker-sync tool to one agent's private format and would need updating
whenever that format changes.

## Last note

`specsync note` appends to a change's `discoveries.md`. Showing its last line adds
"where did I leave off" with no new storage and no agent involvement. For rows with
no change, the field is simply absent.

## Execution model

- Run once per invocation, with a timeout (default 5 seconds, overridable), in the
  directory the user ran specsync from.
- Run through the user's shell because the value is a command line the user wrote
  in their own environment, the same model as `git config core.editor`.
- stdout is parsed with a size cap; stderr is discarded; a non-zero exit, timeout or
  invalid JSON adds one status line and changes nothing else.

## Risks

- A slow provider slows the command. The timeout bounds it and `-sessions-cmd=` (empty)
  disables it for one run.
- A provider could print sensitive session names. specsync prints them only to the
  terminal and persists nothing; the user chose the provider.

## Open questions

- Should `resume` be shown in the default table or only with `-v`? Proposal: show a
  single compact `resume` column in `status`, full `sessions` only in JSON.
