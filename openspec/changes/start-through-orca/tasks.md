# Tasks

- [x] 1.1 `worktrees.launcher` config key (git, orca, auto) and `start -launcher`, `-agent`, `-prompt`
- [x] 1.2 Orca launcher in `StartWork`: `orca worktree create` with name, branch, base, `--no-parent`, issue, agent, prompt
- [x] 1.3 Refuse when Orca created a different branch than asked
- [x] 1.4 Remove any upstream on the new branch
- [x] 1.5 Resolve Orca without ever running the Linux screen reader; `-launcher orca` fails clearly when Orca is missing
- [x] 1.6 `-dry-run` and `-json` report the launcher and its command
- [x] 1.7 Tests with a fake Orca: launch, renamed branch, upstream removed, Linux screen reader, git default unchanged
- [x] 1.8 README, example config, `site/features.json`, skill
- [x] 1.9 Try it with Orca on this repo: Orca 1.4.255 created the workspace on the exact branch from `origin/main`, linked to the issue, with no upstream and the branch link recorded
