# Start, gate and open work the same way every time

## Why

Starting work, checking it and opening a PR are mechanical. They are also easy to do slightly differently each time. A branch is named differently. A gate is skipped. A PR body is written from scratch.

Today a personal script and several skills do this for one person. The general parts belong in the tool. The local conventions belong in config.

## What

Seven verbs. Each does one mechanical thing and refuses unsafe states.

- `specsync new <title>`: create a change from a short template. Check for duplicates first. Show the issue before creating it.
- `specsync start <ref>`: ref is an issue, number, slug or change. Resume the worktree if it exists. Otherwise create worktree and branch from config. Run the start hooks. Record the branch-to-change link. Refuse if a live session sits there.
- `specsync save -m <msg>`: commit, push, sync the spec. Refuse secret-looking files and forbidden trailers. Refuse to push a branch with an open PR unless told to.
- `specsync gates`: run the configured gates in order. Write a stamp for the current commit.
- `specsync pr prep|open`: `prep` checks tasks, freshness, gates stamp and size budget. `open` creates the PR with a generated short body, after explicit confirmations.
- `specsync update`: bring the branch up to date with its base or upstream. It uses the configured strategy and the suggestion from `status`. It prints the plan first. `-apply` runs it. It never force-pushes without `-push`. It refuses on a dirty tree and never forces a shared branch.

Also `specsync park`: save, set stage, leave a note. For stopping work.

Local conventions are config or hooks, never code: worktree directory, branch pattern, env files, install, local services, scratch notes.

## Out of scope

- No agent launching. A harness skill calls these verbs and keeps only the judgement.
- No service manager. Services are a hook.
- No new tracker features. It uses the work provider.

## Migration

A personal script or skill set that does this today can call these verbs and keep only the judgement. The docs include a short guide for that move.

## Related

- `ways-of-working-and-next`: config and `next`. `next` points to these verbs.
- `concise-change-text`: the PR body and issue text.
- `pr-size-advisory`, `review-map-for-prs`: checked in `pr prep`.
- Existing: `pull -worktree`, `archive`, `set-stage`, `audit-tasks`, `pr-body`, `verify`, `note`. These are reused, not replaced.

## Release note

Add `specsync new`, `start`, `save`, `gates`, `pr` and `park`: the same start, check and PR steps every time, configured per repo.
