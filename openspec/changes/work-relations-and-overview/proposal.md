# Show all related work with status, and all work with overview

## Why

A feature spans issues, PRs, branches and worktrees in several repos. Work in one repo depends on work in another.

Today you find the links by hand: epic, sub-issues, PR, branch, worktree. A card for the folder you stand in hides the rest. Hidden work gets forgotten, stalls or collides.

## What

- **A relations graph.** Asserted edges only: epic parent and child, blocked-by and blocks, linked changes, slices of one change, request closes issue, branch to worktree. Each comes from the provider, git or specsync's own files. Never guessed.
- **`specsync status`:** the current work and everything related to it. It climbs to the epic, lists every sibling and shows what blocks what. It marks where you are. Each item shows issue, request, checks, review and, where a worktree is known, local state. It ends with the next step.
- **`specsync overview`:** all work in all repos. Grouped by epic, then work under no epic. Every row has status and relations.
- **Scope the overview.** Flags combine: `-epic <ref>`, `-repo <name>`, `-root <dir>`, `-mine`, `-attention`, `-stage <s>`, `-unattached`. `-json` for agents.
- **Workspace discovery.** Roots come from `-root`, `SPECSYNC_ROOTS`, or by default the parent folder of the current clone. Main clones are found to depth 2. Worktrees come from `git worktree list`, wherever they are. A clone maps to a tracker repo by its origin URL.
- **Each row** carries the same facts as `status`, including whether the base or the upstream moved and whether a rebase would conflict.
- **Work with no change** (a branch and PR only) is shown as unlinked.
- **Branch resolution without naming rules.** The request that closes the issue names the branch. The worktree on that branch belongs to the change.

Example `status`, run in the `api` worktree:

```
api#12  Scope the list                       ◄ you are here
  PR #40 open · review required · checks 9 pass
  local: 2 unpushed · 2 behind main

Epic web#1  Release the picker               3 of 7 done
└─ web#2  Company scope
   ├─ web#5    no PR
   ├─ api#12   PR #40 open                   ◄ you are here
   ├─ worker#9 PR #31 open · behind main 8 · 2 unpushed
   └─ docs#3   not started
Next: push 2 commits, then rebase on main.
```

## Out of scope

- No writes. No inference.
- No scan of the whole disk. Roots are bounded.
- No stored list of paths. No daemon.

## Related

- `status-pr-and-worktree-state`: the facts on each row.
- `status-resume-context`: resume hints on each row.
- `worktree-cleanup`, `ways-of-working-and-next`: read the same graph.
- `issue-dependency-sync`: source of blocked-by. `epic-and-subissue-projection`: source of the epic tree.

## Release note

Add `specsync overview` for all work across repos, and show every related item in `specsync status`.
