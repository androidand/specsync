# Tasks

## 1. Collect the facts

- [ ] 1.1 Add an optional provider capability to list change requests; implement it for GitHub with one `gh pr list --state all --json ...` per repo, mapped by head branch, through the existing `GHRunner` so it is mockable; providers without it are skipped
- [x] 1.2 Add a `LocalState` collector per worktree: modified/untracked counts, upstream, ahead/behind, base and behind-base, size against the merge base, last commit time, volatile-path check
- [ ] 1.3 Resolve the base branch: PR base, else `origin/HEAD`, else omit (`origin/HEAD` and the override are done; the PR base needs the provider)
- [ ] 1.4 Cache the per-repo PR listing with a short TTL via `cache.go`

- [ ] 1.5 Freshness facts: behind upstream, diverged, behind base and when the base moved, overlapping files, shared-branch check
- [ ] 1.6 Conflict prediction with `git merge-tree --write-tree`; unavailable on old git is reported, not guessed
- [ ] 1.7 Request detail from the provider: failing check names, unresolved threads, review started

## 2. Derive and expose

- [ ] 2.0 Sync suggestion from the documented table; exact commands; force-push notice; configured by `sync:`
- [ ] 2.1 (git-only flags done: unpushed, uncommitted, no-upstream, behind-upstream, diverged, behind-base, volatile-path) Compute `attention` from the fixed vocabulary; keep the rules in one table and document them
- [ ] 2.2 (`local` and `attention` done; `pr` needs the provider) Add the optional `pr`, `local` and `attention` fields to `ChangeTopology`; omit when unavailable
- [ ] 2.3 Add `-state=off`; confirm no `gh` process is started in that mode
- [ ] 2.4 Rename `topology` to `status` (no alias; add a habit-word hint). Show the current work from the collector. Related work and `overview` are in `work-relations-and-overview`
- [ ] 2.5 Register `status` in dispatch, `agent-help`, and the unknown-subcommand list

## 3. Tests

- [ ] 3.1 Fixture repo with a mocked runner: open PR, no PR, merged PR with clean tree, truncated listing
- [ ] 3.2a Freshness fixtures: upstream moved, diverged, base moved with and without overlap, predicted conflict, shared branch, review started
- [ ] 3.2b Advice table: one fixture per row, and that nothing is executed
- [ ] 3.2 Git fixtures: ahead, behind, no upstream, dirty, untracked, volatile path
- [ ] 3.3 Determinism: identical facts yield byte-identical output and ordering
- [ ] 3.4 `gh` missing yields rows without `pr`, a status line, and exit 0
- [ ] 3.5 `status -json` golden file shows additive fields only

## 4. Docs

- [ ] 4.1 Implement each new command or flag once, as a `Command` type per `make-commands-discoverable`; README, `agent-help`, the overview and the skill are generated, never hand-written
- [ ] 4.2 Update the skill so agents prefer `status`/`status -json` over hand-run `git`/`gh`
- [ ] 4.3 Update `site/features.json`
- [ ] 4.4 Run `specsync changelog -fail-on-unlinked-commits`
