# Moving a personal script onto the flow verbs

If a script or skill set already starts work, runs checks and opens pull requests, keep only the
judgement in it and call these verbs for the mechanics. Local conventions move to
`openspec/specsync.yml`; nothing project-specific lives in specsync.

| Your script does | Call | Config |
|---|---|---|
| create a worktree and branch, run setup | `specsync start <slug>` | `worktrees.dir`, `worktrees.path`, `worktrees.branch`, `worktrees.hooks`, `worktrees.launcher` |
| write the issue | `specsync new <title>` | the text standard under `text:` |
| commit, refuse secrets and trailers, push | `specsync save -m ... [-push]` | `save.forbid_trailers`, `save.forbid_files` |
| lint, types, tests, then remember it passed | `specsync gates` | `gates:` |
| rebase or merge the base | `specsync update [-apply]` | `sync:` |
| check readiness, open the pull request | `specsync pr prep`, `specsync pr open` | `pr.confirm` |
| stop for the day | `specsync park -m ...` | |

Things to know:

- Commands in the config run only after `specsync trust -yes`; read what `specsync trust` prints first.
- `save` never stages untracked files unless named with `-add`, and never pushes without `-push`.
- `pr open` never pushes and refuses unless `pr prep` passes. Only a failed size check can be waived,
  with `-confirm oversized`.
- Every verb has a dry run, and `specsync next` still names the step you are on.

## Where worktrees go

By default `start` makes `<dir>/<repo>-<slug>` and `pull -worktree` makes `<dir>/<repo>-<issue>`.
Set `worktrees.path` to choose one layout for both:

```yaml
worktrees:
  dir: ../worktrees
  path: "{dir}/{repo}/{name}"   # {name} is <issue>-<slug>, or <slug> without an issue
```

Placeholders: `{dir}`, `{repo}`, `{slug}`, `{issue}`, `{name}`. A template must contain `{slug}` or
`{name}`. With the Orca launcher, Orca gets the template's last folder as the workspace name, so a
layout that Orca also produces stays the same whichever tool creates the worktree.

## Run the gates before every push

`specsync gates -if-stale` runs nothing when the gates already passed for this commit on a clean
tree, and runs them otherwise. Call it from a `pre-push` hook:

```sh
#!/bin/sh
# <git-dir>/hooks/pre-push, or the hooks folder your repo already uses; make it executable
exec specsync gates -if-stale
```

A failed gate stops the push. `git push --no-verify` is git's own way around a hook. `-if-stale`
cannot be combined with `-only`, because a partial run never leaves a stamp.
