# Tasks

## 1. Collect the facts

- [x] 1.1 Add an optional provider capability to list change requests; implement it for GitHub with one `gh pr list --state all --json ...` per repo (page of 100; `commits` is left out of the query because GitHub rejects more than about 40 requests per page with it), mapped by head branch, through the existing `GHRunner` so it is mockable; providers without it are skipped
- [x] 1.2 Add a `LocalState` collector per worktree: modified/untracked counts, upstream, ahead/behind, base and behind-base, size against the merge base, last commit time, volatile-path check
- [x] 1.3 Resolve the base branch: PR base, else `origin/HEAD`, else omit
- [x] 1.4 Cache the per-repo PR listing with a short TTL (a file in the user cache directory, never in a repo; `-state=fresh` bypasses it)

- [x] 1.5 Freshness facts: behind upstream, diverged, behind base and when the base moved, overlapping files, shared-branch check
- [x] 1.6 Conflict prediction with `git merge-tree --write-tree`; unavailable on old git is reported, not guessed
- [x] 1.7 Request detail from the provider: failing check names, unresolved threads, review started (threads come from one GraphQL call; a failed lookup leaves them unknown)

## 2. Derive and expose

- [x] 2.0 Sync suggestion from the documented table; exact commands; force-push notice; configured by `sync:` (takes a `SyncConfig`; reading `sync:` from `specsync.yml` and the `reviewStarted` fact come with `ways-of-working-and-next` and the provider slice)
- [x] 2.1 (done) Compute `attention` from the fixed vocabulary; keep the rules in one table and document them
- [x] 2.2 (done) Add the optional `pr`, `local` and `attention` fields to `ChangeTopology`; omit when unavailable
- [x] 2.3 Add `-state=off`; confirm no `gh` process is started in that mode
- [x] 2.4 Rename `topology` to `status` (no alias; add a habit-word hint). Show the current work from the collector. Related work and `overview` are in `work-relations-and-overview`
- [x] 2.5 Register `status` in dispatch, `agent-help`, and the unknown-subcommand list

## 3. Tests

- [x] 3.1 Fixture repo with a mocked runner: open PR, no PR, merged PR with clean tree, truncated listing
- [x] 3.2a Freshness fixtures: upstream moved, diverged, base moved with and without overlap, predicted conflict, shared branch, review started
- [x] 3.2b Advice table: one fixture per row, and that nothing is executed
- [x] 3.2 Git fixtures: ahead, behind, no upstream, dirty, untracked, volatile path
- [x] 3.3 Determinism: identical facts yield byte-identical output and ordering
- [x] 3.4 `gh` missing yields rows without `pr`, a status line, and exit 0
- [x] 3.5 `status -json` golden file shows additive fields only

## 4. Docs

- [x] 4.1 Implement each new command or flag once, as a `Command` type per `make-commands-discoverable`; README, `agent-help`, the overview and the skill are generated, never hand-written
- [x] 4.2 Update the skill so agents prefer `status`/`status -json` over hand-run `git`/`gh`
- [x] 4.3 Update `site/features.json`
- [x] 4.4 Run `specsync changelog -fail-on-unlinked-commits` (only the pre-existing #198 commit on main is unlinked; none of this change)
