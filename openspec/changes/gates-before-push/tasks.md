# Tasks

## Slice 1: If stale
- contract: `-if-stale` skips a fresh stamp on a clean tree and otherwise behaves as `gates`.
- budget: files<=6, lines<=200

- [x] 1.1 `GateOptions.IfStale` and the flag; refuse it with `-only`
- [x] 1.2 Skip only when the stamp names HEAD and the tree is clean; trust is not needed to skip
- [x] 1.3 Tests: fresh stamp, new commit, dirty tree, `-only` refused, nothing runs when fresh

## Slice 2: Docs
- after: 1
- contract: The pre-push recipe is documented once.

- [x] 2.1 `docs/flow-verbs.md`: the `pre-push` hook recipe
- [ ] 2.2 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
