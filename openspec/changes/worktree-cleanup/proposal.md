# Clean up finished work with specsync cleanup

## Why

Worktrees pile up. Finished work leaves branches, folders and stale changes behind. Each one costs disk and attention. It also hides the work that matters.

The rule is known: remove the worktree when the PR merges. It is still forgotten, because it is manual and per worktree.

`specsync status` already labels finished work `cleanup`. Nothing acts on the label.

## What

`specsync cleanup` lists finished work and, on request, removes it.

- **Dry run by default.** It prints what it would do and why. `-apply` does it.
- **A worktree is a candidate** only when its work is provably in the base: its request is merged and the branch head equals the head the request merged with (nothing was pushed after the merge), or the branch head is an ancestor of the base. It must also be clean. A request closed without merging is never a candidate.
- **Steps per candidate**, each configurable:
  1. remove the worktree
  2. delete the local branch
  3. delete the remote branch (default: keep)
  4. archive the change when its issue is closed
  5. run `cleanup.hooks` (for example stop services or remove notes), only when the config is trusted (see `ways-of-working-and-next`)
- **Never touched:** dirty trees, unpushed commits, open requests, the current worktree, a worktree with a live session.
- **Reported only:** old work that is not provably in the base and has unpushed commits, and branches whose upstream was deleted while their commits are not in the base. It might be lost work. It is never removed.
- `-root <dir>` covers every repo under a directory.

```yaml
cleanup:
  remote_branch: keep     # keep | delete
  archive: true
  min_age_days: 0
  hooks: []
```

`unpushed` has one meaning here: commits not reachable from the base and not on the upstream. When the upstream was deleted (for example by the host after a merge), every commit not reachable from the base counts as unpushed, except a head equal to the merged request's head.

## Out of scope

- No automatic or background cleanup. It runs when invoked.
- No force removal of unsafe work.
- No deleting anything on the tracker.

## Related

- `status-pr-and-worktree-state`: the `cleanup` and `attention` facts.
- `work-relations-and-overview`: workspace discovery for `-root`.
- `status-resume-context`: the live-session check.
- `ways-of-working-and-next`: `next` points to `cleanup` at the end of the flow. This change needs its `cleanup:` config and `trust` first.
- Existing: `archive`.

## Release note

Add `specsync cleanup`: list and remove finished worktrees and branches safely, and archive their changes.
