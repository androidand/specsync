# Moving a personal script onto the flow verbs

If a script or skill set already starts work, runs checks and opens pull requests, keep only the
judgement in it and call these verbs for the mechanics. Local conventions move to
`openspec/specsync.yml`; nothing project-specific lives in specsync.

| Your script does | Call | Config |
|---|---|---|
| create a worktree and branch, run setup | `specsync start <slug>` | `worktrees.dir`, `worktrees.branch`, `worktrees.hooks`, `worktrees.launcher` |
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
