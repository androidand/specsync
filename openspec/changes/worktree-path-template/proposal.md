# Choose where worktrees go with a path template

## Why

`start` puts a worktree at `<dir>/<repo>-<slug>`. That layout is fixed. A repo cannot nest worktrees by repo name or put the issue number in the folder name.

`pull -worktree` has its own fixed layout and ignores the `worktrees:` config.

## What

- A `worktrees.path` template. Placeholders: `{dir}`, `{repo}`, `{slug}`, `{issue}`, `{name}`. `{name}` is `<issue>-<slug>`, or `<slug>` without an issue.
- The default is `{dir}/{repo}-{slug}`, the layout today. Nothing changes unless the key is set.
- `start` and `pull -worktree` share one resolver and read the same config.
- With the Orca launcher and a template, Orca gets the template's last folder name as the workspace name. Without a template it gets the slug, as before.
- Config check: a template must contain `{slug}` or `{name}`, so each change gets its own folder.

## Out of scope

- `slice` keeps its own layout.
- Moving worktrees that already exist.

## Release note

Choose where `start` and `pull -worktree` put worktrees with `worktrees.path`, for example `{dir}/{repo}/{name}`.
