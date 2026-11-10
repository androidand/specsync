# Tasks

Depends on `status-pr-and-worktree-state` (facts) and `ways-of-working-and-next` (`cleanup:` config and `trust`).

## Slice 1: List
- contract: Cleanup lists finished work and why, and changes nothing.
- budget: files<=8, lines<=300

- [x] 1.1 Candidate rule: head equals the merged request's head, or is an ancestor of the base; clean. Closed-unmerged is never a candidate
- [x] 1.1a Define `unpushed` for a deleted upstream; fixtures: closed unmerged, commit after merge, upstream gone
- [x] 1.2 Skip rules: dirty, unpushed, open request, current worktree, live session
- [x] 1.3 "Needs attention" list for old unmerged work with unpushed commits
- [x] 1.4 `specsync cleanup [-root]` dry run with reasons
- [x] 1.5 Tests: each candidate and skip case

## Slice 2: Apply
- after: 1
- contract: With `-apply`, finished work is removed in order. Unsafe work is never touched.
- budget: files<=8, lines<=300

- [x] 2.1 Remove worktree, delete local branch, optional remote branch
- [x] 2.2 Report `specsync archive -change <slug>` as a follow-up when its issue is closed (cleanup never archives itself)
- [x] 2.3 Run `cleanup.hooks` only when the config is trusted; otherwise skip the item and print them (test: untrusted config, mixed trust under `-root`)
- [x] 2.4 `cleanup:` config section, validated by `doctor`
- [x] 2.5 Tests in a fixture repo: removal order, kept remote, hook, archive follow-up

## Slice 3: Docs
- after: 2
- contract: Cleanup is declared once and documented from that declaration.

- [x] 3.1 Declare `cleanup` as a `Command` type; docs are generated
- [x] 3.2 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
