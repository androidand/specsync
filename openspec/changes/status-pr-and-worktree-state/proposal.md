# Show where your work stands with specsync status

## Why

Today the answer to "what state is my work in?" is assembled by hand. It takes `git status`, `git rev-list`, `gh pr view` and `gh pr checks`, once per repo.

Unpushed commits and uncommitted files are the only work lost when a machine dies. Nothing lists them.

`specsync topology` already joins change, issue, branch and worktree. It shows no PR, CI or local state. Its name says nothing to a user. It is unreleased, so it can be renamed now.

## What it shows

Two commands with different jobs.

| Command | Job | Shows |
|---|---|---|
| `specsync status` | where you stand | the current work and everything related to it, with the next step |
| `specsync overview` | everything | all work in all repos, with relations and status, scopable |

This change builds the facts they show: local state, request state and attention flags. `work-relations-and-overview` builds the relations, the related-work view and `overview`.

Run `status` in a worktree. It resolves the branch to its change, then shows that work's request, checks, review and local state. Related work comes from the relations change.

Example freshness lines:

```
Base moved: main is 3 commits ahead. 2 touch files you changed (src/a.go, src/b.go).
Rebase would conflict in src/b.go.
Suggest: merge main (review has started), no force push needed.
```

`-json` gives the same facts for agents.

## What changes

- Rename `topology` to `status`. No alias. It was never released.
- Add a `local` block per worktree: modified and untracked files, ahead and behind, upstream, behind base, size against base, last commit, volatile path.
- Add a `pr` block from the work provider: state, draft, review, mergeable, checks, issues closed.
- Add freshness facts. Has the base moved? Has the branch's own upstream moved? Do they diverge? Do the base's new commits touch files you changed? Would a rebase conflict? All read-only. The conflict check uses `git merge-tree` and never touches the worktree.
- Add a sync suggestion: a strategy (fast-forward, rebase, merge) with the exact commands, and whether a force push would be needed. It is advice. `status` never runs it.
- Add request detail: failing check names, unresolved review threads, and a `ready-to-merge` signal.
- Add `attention` flags: `unpushed`, `uncommitted`, `no-upstream`, `behind-upstream`, `diverged`, `behind-base`, `base-touches-your-files`, `conflicts-with-base`, `ci-failing`, `changes-requested`, `review-missing`, `unresolved-comments`, `volatile-path`, `cleanup`. Plus the positive `ready-to-merge`.
- `-state=off` skips the provider. Local facts only, instant.
- Any source that fails narrows a row. It never fails the command.

## Out of scope

- No writes. `cleanup` is a label, not an action.
- No inference. Every field comes from git or the provider.
- No daemon. No new state beyond the existing short-lived cache.
- No verdict on size. See `pr-size-advisory`.

## Works anywhere

- Local facts need only git. Request facts come from the work provider.
- No branch or directory naming is assumed.
- Humans read the table. Any agent reads `-json`.

## Related

- `work-graph`: shares the `gh` runner. Adds no commit or release edges.
- `in-flight-work-claims`: records intent. `status` reports observed facts.
- `work-relations-and-overview`: relations, the related-work view in `status`, and `overview`.

## Release note

Add `specsync status`: the state of your current work, with PR, checks, review and local state.
