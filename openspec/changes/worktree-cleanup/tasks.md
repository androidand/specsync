# Tasks

Depends on `status-pr-and-worktree-state`.

## Slice 1: List
- contract: Cleanup lists finished work and why, and changes nothing.
- budget: files<=8, lines<=300

- [ ] 1.1 Candidate rule: merged or closed request, or branch merged into base; clean; nothing unpushed
- [ ] 1.2 Skip rules: dirty, unpushed, open request, current worktree, live session
- [ ] 1.3 "Needs attention" list for old unmerged work with unpushed commits
- [ ] 1.4 `specsync cleanup [-root]` dry run with reasons
- [ ] 1.5 Tests: each candidate and skip case

## Slice 2: Apply
- after: 1
- contract: With `-apply`, finished work is removed in order. Unsafe work is never touched.
- budget: files<=8, lines<=300

- [ ] 2.1 Remove worktree, delete local branch, optional remote branch
- [ ] 2.2 Archive the change when its issue is closed, using `archive`
- [ ] 2.3 Run `cleanup.hooks`
- [ ] 2.4 `cleanup:` config section, validated by `doctor`
- [ ] 2.5 Tests in a fixture repo: removal order, kept remote, hook, archive

## Slice 3: Docs
- after: 2
- contract: Cleanup is declared once and documented from that declaration.

- [ ] 3.1 Declare `cleanup` as a `Command` type; docs are generated
- [ ] 3.2 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
