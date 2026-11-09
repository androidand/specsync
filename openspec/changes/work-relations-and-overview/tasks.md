# Tasks

Depends on `status-pr-and-worktree-state` for facts per row.

## Slice 1: Discovery and graph
- contract: The workspace is found and the asserted relations graph is built, with no writes.
- budget: files<=15, lines<=500

- [ ] 1.1 Roots: `-root`, `SPECSYNC_ROOTS`, default parent folder of the clone
- [ ] 1.2 Find main clones to depth 2; list worktrees with `git worktree list --porcelain`; dedupe by real path
- [ ] 1.3 Map clones to tracker repos from the origin URL; status line when unmappable
- [ ] 1.4 Graph with asserted edges only: epic, blocked-by, links, slices, request closes issue, branch to worktree
- [ ] 1.5 Reuse the provider's parent and child client; fall back to specsync's epic references
- [ ] 1.6 Extract the PR-reference parser from `verify` into a shared function
- [ ] 1.7 Resolve a change's branch through the request that closes its issue

## Slice 2: Status shows related work
- after: 1
- contract: `status` shows the current work and everything related to it, marked, with the next step.
- budget: files<=10, lines<=400

- [ ] 2.1 Climb to the epic; list siblings, blockers and blocked work; mark the current item
- [ ] 2.2 Show issue, request, checks, review and local state per item
- [ ] 2.3 Fall back to the repo's ongoing changes outside a worktree
- [ ] 2.4 Mark sections incomplete when a source fails

## Slice 3: Overview and scoping
- after: 1
- contract: `overview` lists all work with relations and supports combinable scopes.
- budget: files<=10, lines<=400

- [ ] 3.1 `specsync overview`: group by epic, then work under no epic, then unlinked branches
- [ ] 3.2 Flags: `-epic`, `-repo`, `-root`, `-mine`, `-attention`, `-stage`, `-unattached`
- [ ] 3.3 Print the applied scope; `-json` with the nested structure
- [ ] 3.4 Rows for work with no change, marked unlinked

## Slice 4: Tests and docs
- after: 2, 3
- contract: Behaviour is pinned by fixtures and documented from the declarations.

- [ ] 4.1 Fixture workspace: clones, a worktree in a temp folder, a clone with no origin
- [ ] 4.2 Epic fixture across repos with a mocked provider; no-relations and not-in-worktree cases
- [ ] 4.3 Scope combinations; unlinked work; incomplete markers
- [ ] 4.4 Read-only guarantee test
- [ ] 4.5 Declare `status` and `overview` once as `Command` types; docs are generated
- [ ] 4.6 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
