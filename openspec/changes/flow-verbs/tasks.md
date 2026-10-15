# Tasks

Depends on `ways-of-working-and-next` slice 1 for config.

## Slice 1: Gates
- contract: Gates run in order and leave a stamp that goes stale on a new commit.
- budget: files<=8, lines<=300

- [ ] 1.1 `specsync gates [--only a,b] [--with x]`
- [ ] 1.2 Stamp tied to the head commit; invalid after a new commit
- [ ] 1.3 Tests: order, optional gates, failure, stale stamp

## Slice 2: Start and new
- after: 1
- contract: Start creates or resumes the same worktree the same way every time. New never creates without confirmation.
- budget: files<=15, lines<=500

- [ ] 2.1 `specsync start <ref>`: resolve, resume or create from config, run hooks, record the branch link
- [ ] 2.2 Refuse when a session provider reports a live session
- [ ] 2.3 `specsync new <title>` with the short template and a duplicate check
- [ ] 2.4 Build on `pull -worktree` instead of duplicating it
- [ ] 2.5 Tests: existing worktree, new worktree, hooks, refusal, duplicate shown

## Slice 3: Save and park
- after: 1
- contract: Save refuses unsafe commits and pushes. Park leaves the work resumable.
- budget: files<=10, lines<=400

- [ ] 3.1 `specsync save -m`: commit, guarded push, spec sync
- [ ] 3.2 Guards: secret-looking files, forbidden trailers, open-request push
- [ ] 3.3 `specsync park`: save, stage, note
- [ ] 3.4 Tests for each guard

## Slice 4: Pull request
- after: 1
- contract: PR prep fails with a reason. PR open needs confirmations and uses a short generated body.
- budget: files<=12, lines<=450

- [ ] 4.1 `pr prep`: tasks, freshness, stamp, size budget
- [ ] 4.2 `pr open` with confirmations from config and the body from `concise-change-text`
- [ ] 4.3 Tests: each failing check, missing confirmation, success

## Slice 5: Keep branches current
- after: 1
- contract: Update brings a branch current by the configured strategy, with a plan first and no unsafe rewrites.
- budget: files<=8, lines<=350

- [ ] 5.1 `specsync update [-apply] [-push]` using the sync suggestion and `sync:` config
- [ ] 5.2 Refuse on dirty trees; never force a shared branch; push only with `-push`, with a lease
- [ ] 5.3 Stop cleanly at a conflict and say which files
- [ ] 5.4 Tests: fast-forward, rebase, merge, conflict stop, shared branch, dirty tree

## Slice 6: Docs
- after: 2, 3, 4, 5
- contract: Every verb is declared once and documented from that declaration.

- [ ] 6.1 Declare each verb as a `Command` type; docs are generated
- [ ] 6.2 Short migration guide for people moving from a personal script
- [ ] 6.3 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
