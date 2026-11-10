# Session provider (optional)

`specsync status` and `specsync overview` can show how to resume each piece of work. Without a provider every row with a worktree gets `cd <worktree>` and, for a change, the last line of its `discoveries.md` (`specsync note` writes it). A provider adds the agent session that was working there.

specsync knows no agent's files. A provider is any command that prints a JSON array:

```json
[{"cwd": "/path/to/worktree", "id": "s-123", "name": "api-scope",
  "status": "idle", "updatedAt": "2026-10-13T09:12:00Z",
  "resume": "agent --resume s-123"}]
```

Only `cwd` is required. A session attaches to the row whose worktree is the longest path prefix of its `cwd`. `resume` is printed as is and never interpreted; the row's `resume` is that of the most recently updated session.

```bash
specsync status -sessions-cmd ./my-sessions.sh
SPECSYNC_SESSIONS_CMD=./my-sessions.sh specsync overview
specsync status -sessions-cmd=          # disable for one run
```

The command comes only from the flag or the environment of the person running specsync, never from a file in a repository. It runs once through `sh -c`, with a timeout (`-sessions-timeout`, default 5 s) and a 1 MiB output cap. A failure, timeout or invalid JSON adds one status line and changes nothing else. specsync stores nothing about sessions.

## Example adapter (agent-specific, optional)

For an agent that keeps one JSON file per session in a folder, with `cwd`, `id`, `updated` and a resume command inside. The file layout is that agent's, not specsync's.

```sh
#!/bin/sh
dir="${AGENT_SESSIONS_DIR:-$HOME/.agent/sessions}"
jq -s '[.[] | {cwd, id, name: .title, updatedAt: .updated, resume: ("agent --resume " + .id)}]' "$dir"/*.json
```
